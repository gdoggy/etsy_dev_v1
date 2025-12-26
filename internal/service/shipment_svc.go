package service

import (
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"fmt"
	"time"

	"gorm.io/datatypes"
)

// 确保 ShipmentService 实现 ShipmentTracker 接口
var _ interface {
	RefreshTracking(ctx context.Context, shipmentID int64) error
	SyncToEtsy(ctx context.Context, shipmentID int64) error
} = (*ShipmentService)(nil)

// ==================== 外部依赖接口 ====================

// EtsyShipmentSyncer Etsy 发货同步接口
type EtsyShipmentSyncer interface {
	CreateReceipt(ctx context.Context, shopID int64, receiptID int64, trackingCode, carrierName string) error
}

// ==================== Service 实现 ====================

// ShipmentService 发货服务
type ShipmentService struct {
	shipmentRepo repository.ShipmentRepository
	eventRepo    repository.TrackingEventRepository
	orderRepo    repository.OrderRepository
	shopRepo     repository.ShopRepository
	karrio       *KarrioClient // 使用同包的 KarrioClient（定义于 karrio_svc.go）
	etsySyncer   EtsyShipmentSyncer

	// 物流商映射
	carrierNames map[string]string
}

// NewShipmentService 创建发货服务
func NewShipmentService(
	shipmentRepo repository.ShipmentRepository,
	eventRepo repository.TrackingEventRepository,
	orderRepo repository.OrderRepository,
	shopRepo repository.ShopRepository,
	karrio *KarrioClient,
	etsySyncer EtsyShipmentSyncer,
) *ShipmentService {
	return &ShipmentService{
		shipmentRepo: shipmentRepo,
		eventRepo:    eventRepo,
		orderRepo:    orderRepo,
		shopRepo:     shopRepo,
		karrio:       karrio,
		etsySyncer:   etsySyncer,
		carrierNames: map[string]string{
			"yanwen":     "燕文物流",
			"wanbang":    "万邦速达",
			"cainiao":    "菜鸟物流",
			"yto":        "圆通国际",
			"sto":        "申通国际",
			"zto":        "中通国际",
			"sf":         "顺丰国际",
			"dhl":        "DHL",
			"fedex":      "FedEx",
			"ups":        "UPS",
			"usps":       "USPS",
			"royal_mail": "Royal Mail",
		},
	}
}

// ==================== 发货管理 ====================

// CreateShipment 创建发货记录
func (s *ShipmentService) CreateShipment(ctx context.Context, orderID int64, carrierCode, serviceCode, trackingNumber string, weight float64) (*model.Shipment, error) {
	// 检查订单
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 检查是否已有发货记录
	existing, _ := s.shipmentRepo.GetByOrderID(ctx, orderID)
	if existing != nil {
		return nil, fmt.Errorf("订单已有发货记录")
	}

	// 检查订单状态
	if !order.CanShip() && order.Status != model.OrderStatusPending {
		return nil, fmt.Errorf("订单状态不允许发货")
	}

	// 获取物流商名称
	carrierName := s.getCarrierName(carrierCode)

	// 创建发货记录
	now := time.Now()
	shipment := &model.Shipment{
		OrderID:        orderID,
		CarrierCode:    carrierCode,
		CarrierName:    carrierName,
		ServiceCode:    serviceCode,
		TrackingNumber: trackingNumber,
		Weight:         weight,
		WeightUnit:     "KG",
		Status:         model.ShipmentStatusCreated,
		ShippedAt:      &now,
	}

	if err := s.shipmentRepo.Create(ctx, shipment); err != nil {
		return nil, fmt.Errorf("创建发货记录失败: %v", err)
	}

	// 更新订单状态
	s.orderRepo.UpdateFields(ctx, orderID, map[string]interface{}{
		"status":     model.OrderStatusShipped,
		"is_shipped": true,
		"shipped_at": &now,
	})

	// 如果有跟踪号，创建 Karrio Tracker
	if trackingNumber != "" && s.karrio != nil {
		go s.createTracker(context.Background(), shipment.ID, carrierCode, trackingNumber)
	}

	return shipment, nil
}

// CreateShipmentWithLabel 通过 Karrio 创建发货并获取面单
func (s *ShipmentService) CreateShipmentWithLabel(ctx context.Context, orderID int64, carrierCode, serviceCode string) (*model.Shipment, error) {
	// 检查订单
	order, err := s.orderRepo.GetByIDWithRelations(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在: %v", err)
	}

	// 检查 Karrio 客户端
	if s.karrio == nil {
		return nil, fmt.Errorf("Karrio 客户端未配置")
	}

	// 构建收件人地址
	recipient := s.buildRecipientAddress(order)

	// 构建发件人地址（从店铺配置获取）
	shop, err := s.shopRepo.GetByID(ctx, order.ShopID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺失败: %v", err)
	}
	shipper := s.buildShipperAddress(shop)

	// 计算包裹信息
	parcel := s.calculateParcel(order)

	// 调用 Karrio 创建发货（使用 dto 类型）
	karrioReq := &dto.CreateShipmentRequest{
		CarrierName: carrierCode,
		ServiceCode: serviceCode,
		Shipper:     shipper,
		Recipient:   recipient,
		Parcels:     []dto.Parcel{parcel},
	}

	karrioResp, err := s.karrio.CreateShipment(ctx, karrioReq)
	if err != nil {
		return nil, fmt.Errorf("Karrio 创建发货失败: %v", err)
	}

	// 创建发货记录
	now := time.Now()
	shipment := &model.Shipment{
		OrderID:          orderID,
		KarrioShipmentID: karrioResp.ID,
		CarrierCode:      carrierCode,
		CarrierName:      s.getCarrierName(carrierCode),
		ServiceCode:      serviceCode,
		TrackingNumber:   karrioResp.TrackingNumber,
		LabelURL:         karrioResp.LabelURL,
		LabelType:        karrioResp.LabelType,
		Weight:           parcel.Weight,
		WeightUnit:       parcel.WeightUnit,
		Status:           model.ShipmentStatusCreated,
		ShippedAt:        &now,
	}

	if err := s.shipmentRepo.Create(ctx, shipment); err != nil {
		return nil, fmt.Errorf("保存发货记录失败: %v", err)
	}

	// 更新订单状态
	s.orderRepo.UpdateFields(ctx, orderID, map[string]interface{}{
		"status":     model.OrderStatusShipped,
		"is_shipped": true,
		"shipped_at": &now,
	})

	// 创建 Karrio Tracker
	go s.createTracker(context.Background(), shipment.ID, carrierCode, karrioResp.TrackingNumber)

	return shipment, nil
}

// createTracker 创建 Karrio 跟踪器
func (s *ShipmentService) createTracker(ctx context.Context, shipmentID int64, carrierCode, trackingNumber string) {
	if s.karrio == nil {
		return
	}

	req := &dto.CreateTrackerRequest{
		TrackingNumber: trackingNumber,
		CarrierName:    carrierCode,
	}

	resp, err := s.karrio.CreateTracker(ctx, req)
	if err != nil {
		return
	}

	// 更新 Karrio Tracker ID
	s.shipmentRepo.UpdateFields(ctx, shipmentID, map[string]interface{}{
		"karrio_tracker_id": resp.ID,
	})
}

// ==================== 查询 ====================

// GetByID 获取发货详情
func (s *ShipmentService) GetByID(ctx context.Context, id int64) (*model.Shipment, error) {
	return s.shipmentRepo.GetByIDWithEvents(ctx, id)
}

// GetByOrderID 根据订单获取发货
func (s *ShipmentService) GetByOrderID(ctx context.Context, orderID int64) (*model.Shipment, error) {
	return s.shipmentRepo.GetByOrderID(ctx, orderID)
}

// List 列表查询
func (s *ShipmentService) List(ctx context.Context, filter repository.ShipmentFilter) ([]model.Shipment, int64, error) {
	return s.shipmentRepo.List(ctx, filter)
}

// ==================== 物流跟踪 ====================

// RefreshTracking 刷新物流跟踪
func (s *ShipmentService) RefreshTracking(ctx context.Context, shipmentID int64) error {
	shipment, err := s.shipmentRepo.GetByID(ctx, shipmentID)
	if err != nil {
		return fmt.Errorf("发货记录不存在")
	}

	if shipment.KarrioTrackerID == "" {
		return fmt.Errorf("无跟踪器")
	}

	if s.karrio == nil {
		return fmt.Errorf("Karrio 客户端未配置")
	}

	// 刷新并获取最新跟踪信息
	tracker, err := s.karrio.RefreshTracker(ctx, shipment.KarrioTrackerID)
	if err != nil {
		return fmt.Errorf("获取跟踪信息失败: %v", err)
	}

	// 更新状态
	newStatus := s.mapKarrioStatus(tracker.Delivered)
	if newStatus != shipment.Status {
		s.shipmentRepo.UpdateStatus(ctx, shipmentID, newStatus)
	}

	// 保存事件
	s.saveTrackingEvents(ctx, shipmentID, tracker.Events)

	// 更新最新跟踪信息
	if len(tracker.Events) > 0 {
		latest := tracker.Events[0]
		eventTime := s.parseEventTime(latest.Date, latest.Time)
		s.shipmentRepo.UpdateTrackingInfo(ctx, shipmentID, latest.Description, latest.Location, eventTime)
	}

	return nil
}

// HandleWebhook 处理 Karrio Webhook
func (s *ShipmentService) HandleWebhook(ctx context.Context, trackerID string, status string, events []dto.TrackingEvent) error {
	// 查找发货记录
	shipment, err := s.shipmentRepo.GetByKarrioTrackerID(ctx, trackerID)
	if err != nil {
		return fmt.Errorf("发货记录不存在: tracker_id=%s", trackerID)
	}

	// 更新状态
	newStatus := s.mapKarrioStatus(status)
	if newStatus != shipment.Status {
		s.shipmentRepo.UpdateStatus(ctx, shipment.ID, newStatus)
	}

	// 保存事件
	s.saveTrackingEvents(ctx, shipment.ID, events)

	// 更新最新跟踪信息
	if len(events) > 0 {
		latest := events[0]
		eventTime := s.parseEventTime(latest.Date, latest.Time)
		s.shipmentRepo.UpdateTrackingInfo(ctx, shipment.ID, latest.Description, latest.Location, eventTime)
	}

	return nil
}

// saveTrackingEvents 保存跟踪事件
func (s *ShipmentService) saveTrackingEvents(ctx context.Context, shipmentID int64, events []dto.TrackingEvent) {
	if len(events) == 0 {
		return
	}

	var trackingEvents []model.TrackingEvent
	for _, e := range events {
		eventTime := s.parseEventTime(e.Date, e.Time)
		if eventTime == nil {
			continue
		}

		rawPayload, _ := json.Marshal(e)

		trackingEvents = append(trackingEvents, model.TrackingEvent{
			ShipmentID:  shipmentID,
			OccurredAt:  *eventTime,
			Status:      e.Description,
			StatusCode:  e.Code,
			Description: e.Description,
			Location:    e.Location,
			RawPayload:  datatypes.JSON(rawPayload),
		})
	}

	if len(trackingEvents) > 0 {
		s.eventRepo.CreateBatch(ctx, trackingEvents)
	}
}

// ==================== Etsy 同步 ====================

// SyncToEtsy 同步发货信息到 Etsy
func (s *ShipmentService) SyncToEtsy(ctx context.Context, shipmentID int64) error {
	shipment, err := s.shipmentRepo.GetByID(ctx, shipmentID)
	if err != nil {
		return fmt.Errorf("发货记录不存在")
	}

	if shipment.EtsySynced {
		return nil // 已同步
	}

	if shipment.TrackingNumber == "" {
		return fmt.Errorf("无跟踪号")
	}

	// 获取订单
	order, err := s.orderRepo.GetByID(ctx, shipment.OrderID)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}

	if s.etsySyncer == nil {
		return fmt.Errorf("Etsy 同步器未配置")
	}

	// 同步到 Etsy
	// todo. 验证需要同步到 etsy 的 TrackingNumber & CarrierName
	err = s.etsySyncer.CreateReceipt(ctx, order.ShopID, order.EtsyReceiptID, shipment.TrackingNumber, shipment.CarrierName)
	if err != nil {
		s.shipmentRepo.MarkEtsySyncFailed(ctx, shipmentID, err.Error())
		return fmt.Errorf("Etsy 同步失败: %v", err)
	}

	// 标记已同步
	s.shipmentRepo.MarkEtsySynced(ctx, shipmentID)

	return nil
}

// ==================== 辅助方法 ====================

func (s *ShipmentService) getCarrierName(code string) string {
	if name, ok := s.carrierNames[code]; ok {
		return name
	}
	return code
}

func (s *ShipmentService) mapKarrioStatus(status string) string {
	switch status {
	case "in_transit":
		return model.ShipmentStatusInTransit
	case "out_for_delivery":
		return model.ShipmentStatusDelivering
	case "delivered":
		return model.ShipmentStatusDelivered
	case "failure", "exception":
		return model.ShipmentStatusException
	case "returned":
		return model.ShipmentStatusReturned
	default:
		return model.ShipmentStatusCreated
	}
}

func (s *ShipmentService) parseEventTime(date, timeStr string) *time.Time {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	combined := date
	if timeStr != "" {
		combined = date + " " + timeStr
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, combined); err == nil {
			return &t
		}
	}
	return nil
}

func (s *ShipmentService) buildRecipientAddress(order *model.Order) *dto.Address {
	return &dto.Address{
		PersonName:   order.GetShippingAddressField("name"),
		AddressLine1: order.GetShippingAddressField("first_line"),
		AddressLine2: order.GetShippingAddressField("second_line"),
		City:         order.GetShippingAddressField("city"),
		StateCode:    order.GetShippingAddressField("state"),
		PostalCode:   order.GetShippingAddressField("zip"),
		CountryCode:  order.GetShippingAddressField("country_iso"),
	}
}

func (s *ShipmentService) buildShipperAddress(shop *model.Shop) *dto.Address {
	// 从店铺配置获取发货地址
	// 实际项目中应该从店铺配置或系统配置获取
	return &dto.Address{
		PersonName:   "Etsy Seller",
		AddressLine1: "Shipping Address",
		City:         "Shenzhen",
		StateCode:    "GD",
		PostalCode:   "518000",
		CountryCode:  "CN",
	}
}

func (s *ShipmentService) calculateParcel(order *model.Order) dto.Parcel {
	// 简单计算，实际应该根据商品信息计算
	return dto.Parcel{
		Weight:     0.5, // 默认 0.5kg
		WeightUnit: "KG",
	}
}

// ==================== 物流商管理 ====================

// GetSupportedCarriers 获取支持的物流商
func (s *ShipmentService) GetSupportedCarriers() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"code": "yanwen",
			"name": "燕文物流",
			"services": []map[string]string{
				{"code": "yanwen_express", "name": "燕文特快"},
				{"code": "yanwen_economy", "name": "燕文经济"},
			},
		},
		{
			"code": "wanbang",
			"name": "万邦速达",
			"services": []map[string]string{
				{"code": "wanbang_standard", "name": "万邦标准"},
				{"code": "wanbang_express", "name": "万邦特快"},
			},
		},
		{
			"code": "cainiao",
			"name": "菜鸟物流",
			"services": []map[string]string{
				{"code": "cainiao_standard", "name": "菜鸟标准"},
			},
		},
	}
}
