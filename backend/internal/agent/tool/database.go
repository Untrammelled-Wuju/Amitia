package tool

import "database/sql"

var toolDB *sql.DB

func SetDB(db *sql.DB) { toolDB = db }
