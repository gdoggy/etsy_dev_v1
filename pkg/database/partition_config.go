package database

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PartitionTableConfig 分区表配置
type PartitionTableConfig struct {
	TableName      string // 表名
	RetentionMonth int    // 保留月数（0=永久）
	SQLContent     string // SQL 内容
}

// PartitionConfig 分区配置
type PartitionConfig struct {
	Tables []PartitionTableConfig
}

// LoadPartitionConfig 从嵌入文件系统加载配置
func LoadPartitionConfig(embedFS embed.FS, root string) (*PartitionConfig, error) {
	cfg := &PartitionConfig{}

	// 读取配置文件
	confPath := filepath.Join(root, "partition_tables.conf")
	confData, err := embedFS.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	if err := cfg.parseConfig(string(confData)); err != nil {
		return nil, err
	}

	// 加载 SQL 内容
	for i := range cfg.Tables {
		sqlFile := cfg.Tables[i].TableName + ".sql"
		sqlPath := filepath.Join(root, sqlFile)
		sqlData, err := embedFS.ReadFile(sqlPath)
		if err != nil {
			return nil, fmt.Errorf("读取 SQL 文件 %s 失败: %w", sqlFile, err)
		}
		cfg.Tables[i].SQLContent = string(sqlData)
	}

	return cfg, nil
}

// LoadPartitionConfigFromDir 从目录加载配置
func LoadPartitionConfigFromDir(dir string) (*PartitionConfig, error) {
	cfg := &PartitionConfig{}

	// 读取配置文件
	confPath := filepath.Join(dir, "partition_tables.conf")
	confData, err := os.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	if err := cfg.parseConfig(string(confData)); err != nil {
		return nil, err
	}

	// 加载 SQL 内容
	for i := range cfg.Tables {
		sqlFile := cfg.Tables[i].TableName + ".sql"
		sqlPath := filepath.Join(dir, sqlFile)
		sqlData, err := os.ReadFile(sqlPath)
		if err != nil {
			return nil, fmt.Errorf("读取 SQL 文件 %s 失败: %w", sqlFile, err)
		}
		cfg.Tables[i].SQLContent = string(sqlData)
	}

	return cfg, nil
}

// parseConfig 解析配置内容
func (c *PartitionConfig) parseConfig(content string) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return fmt.Errorf("配置第 %d 行格式错误: %s", lineNum, line)
		}

		retention, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("配置第 %d 行保留月数无效: %s", lineNum, parts[1])
		}

		c.Tables = append(c.Tables, PartitionTableConfig{
			TableName:      strings.TrimSpace(parts[0]),
			RetentionMonth: retention,
		})
	}

	return scanner.Err()
}

// GetTableNames 获取所有分区表名
func (c *PartitionConfig) GetTableNames() []string {
	names := make([]string, len(c.Tables))
	for i, t := range c.Tables {
		names[i] = t.TableName
	}
	return names
}

// GetTable 获取指定表配置
func (c *PartitionConfig) GetTable(name string) *PartitionTableConfig {
	for i := range c.Tables {
		if c.Tables[i].TableName == name {
			return &c.Tables[i]
		}
	}
	return nil
}

// IsPartitionedTable 检查是否为分区表
func (c *PartitionConfig) IsPartitionedTable(name string) bool {
	return c.GetTable(name) != nil
}

// WalkSQLFiles 遍历目录中的 SQL 文件（用于外部加载）
func WalkSQLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
