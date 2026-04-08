package snapshot

import "time"

// SnapshotHeader 保存快照包中返回的快照基本标识字段。
type SnapshotHeader struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
}

// SnapshotListItem 保存用于列表展示的快照摘要数据。
type SnapshotListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
	FileCount int       `json:"file_count"`
}
