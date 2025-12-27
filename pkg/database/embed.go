package database

import "embed"

// PartitionSQL 嵌入分区 SQL 文件
//
//go:embed partitions/*.sql partitions/*.conf
var PartitionSQL embed.FS
