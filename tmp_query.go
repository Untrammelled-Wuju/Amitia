package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "D:\\桌面\\跟进项目\\U-Ai\\backend\\data\\app.db")
	if err != nil { fmt.Println("Open error:", err); return }
	defer db.Close()

	// 查询所有对话
	rows, err := db.Query("SELECT id, title, channel, character_id FROM conversations ORDER BY channel, created_at DESC")
	if err != nil { fmt.Println("Query error:", err); return }
	defer rows.Close()

	type Conv struct { ID, Title, Channel, CID string }
	var convs []Conv
	for rows.Next() {
		var c Conv
		rows.Scan(&c.ID, &c.Title, &c.Channel, &c.CID)
		convs = append(convs, c)
	}

	// 找出重复的QQ/微信对话
	wechat := []Conv{}
	qq := []Conv{}
	orphan := []Conv{}
	for _, c := range convs {
		if c.Channel == "wechat" { wechat = append(wechat, c) } else
		if c.Channel == "qq" { qq = append(qq, c) } else
		if c.CID == "" || c.CID == "0" { orphan = append(orphan, c) }
	}

	fmt.Println("=== 微信对话 (channel=wechat) ===")
	for _, c := range wechat { fmt.Printf("  id=%s title=%s char=%s\n", c.ID[:12], c.Title, c.CID) }
	fmt.Println("=== QQ对话 (channel=qq) ===")
	for _, c := range qq { fmt.Printf("  id=%s title=%s char=%s\n", c.ID[:12], c.Title, c.CID) }
	fmt.Println("=== 非角色对话 (character_id为空) ===")
	for _, c := range orphan { fmt.Printf("  id=%s title=%s channel=%s\n", c.ID[:12], c.Title, c.Channel) }
}
