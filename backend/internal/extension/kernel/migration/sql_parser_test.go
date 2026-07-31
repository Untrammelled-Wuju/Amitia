package migration

import (
	"errors"
	"testing"
)

func assertStmtType(t *testing.T, stmt *SQLStatement, expectedType SQLStatementType) {
	t.Helper()
	if stmt.Type != expectedType {
		t.Errorf("期望语句类型 %s，实际 %s", expectedType, stmt.Type)
	}
}

func assertObjectNames(t *testing.T, stmt *SQLStatement, expected ...string) {
	t.Helper()
	names := stmt.AllObjectNames()
	if len(names) != len(expected) {
		t.Errorf("期望对象数量 %d，实际 %d（%v）", len(expected), len(names), names)
		return
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("第 %d 个对象期望名称 %s，实际 %s", i, name, names[i])
		}
	}
}

func assertObjectKinds(t *testing.T, stmt *SQLStatement, expected ...string) {
	t.Helper()
	if len(stmt.Objects) != len(expected) {
		t.Errorf("期望对象数量 %d，实际 %d", len(expected), len(stmt.Objects))
		return
	}
	for i, kind := range expected {
		if stmt.Objects[i].Kind != kind {
			t.Errorf("第 %d 个对象期望种类 %s，实际 %s", i, kind, stmt.Objects[i].Kind)
		}
	}
}

func TestParseStatement_CreateTable(t *testing.T) {
	t.Run("普通建表语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TABLE ext_test_table (id INTEGER PRIMARY KEY)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("IF NOT EXISTS 建表语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TABLE IF NOT EXISTS ext_test_table (id INTEGER PRIMARY KEY)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("临时表建表语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TEMP TABLE ext_temp_table (id INTEGER)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_temp_table")
	})

	t.Run("临时表关键字 TEMPORARY", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TEMPORARY TABLE ext_temp2 (id INTEGER)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_temp2")
	})
}

func TestParseStatement_CreateView(t *testing.T) {
	t.Run("普通建视图语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE VIEW ext_test_view AS SELECT * FROM ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateView)
		assertObjectNames(t, stmt, "ext_test_view", "ext_test_table")
		assertObjectKinds(t, stmt, "view", "table")
	})

	t.Run("IF NOT EXISTS 建视图语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE VIEW IF NOT EXISTS ext_test_view AS SELECT * FROM ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateView)
		assertObjectNames(t, stmt, "ext_test_view", "ext_test_table")
	})

	t.Run("多表视图语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE VIEW ext_join_view AS SELECT a.id FROM ext_table_a a JOIN ext_table_b b ON a.id = b.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateView)
		assertObjectNames(t, stmt, "ext_join_view", "ext_table_a", "ext_table_b")
	})
}

func TestParseStatement_CreateTrigger(t *testing.T) {
	t.Run("BEFORE INSERT 触发器", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TRIGGER ext_test_trigger BEFORE INSERT ON ext_test_table BEGIN END")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTrigger)
		assertObjectNames(t, stmt, "ext_test_trigger", "ext_test_table")
		assertObjectKinds(t, stmt, "trigger", "table")
	})

	t.Run("AFTER UPDATE 触发器", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TRIGGER ext_after_trigger AFTER UPDATE ON ext_test_table BEGIN END")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTrigger)
		assertObjectNames(t, stmt, "ext_after_trigger", "ext_test_table")
	})

	t.Run("INSTEAD OF 触发器", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TRIGGER ext_instead_trigger INSTEAD OF DELETE ON ext_test_view BEGIN END")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTrigger)
		assertObjectNames(t, stmt, "ext_instead_trigger", "ext_test_view")
	})

	t.Run("FOR EACH ROW 触发器", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TRIGGER ext_row_trigger BEFORE INSERT ON ext_test_table FOR EACH ROW BEGIN END")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTrigger)
		assertObjectNames(t, stmt, "ext_row_trigger", "ext_test_table")
	})

	t.Run("复杂触发器含多条语句", func(t *testing.T) {
		raw := `CREATE TRIGGER ext_complex_trigger BEFORE INSERT ON ext_test_table
BEGIN
    INSERT INTO ext_log VALUES (1);
    UPDATE ext_log SET msg = 'trigger';
END`
		stmt, err := ParseStatement(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTrigger)
		assertObjectNames(t, stmt, "ext_complex_trigger", "ext_test_table", "ext_log")
		assertObjectKinds(t, stmt, "trigger", "table", "table")
	})
}

func TestParseStatement_CreateIndex(t *testing.T) {
	t.Run("普通建索引语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE INDEX ext_test_idx ON ext_test_table (id)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateIndex)
		assertObjectNames(t, stmt, "ext_test_idx", "ext_test_table")
		assertObjectKinds(t, stmt, "index", "table")
	})

	t.Run("唯一索引语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE UNIQUE INDEX ext_test_uidx ON ext_test_table (id)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateIndex)
		assertObjectNames(t, stmt, "ext_test_uidx", "ext_test_table")
		assertObjectKinds(t, stmt, "index", "table")
	})

	t.Run("IF NOT EXISTS 建索引语句", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE INDEX IF NOT EXISTS ext_test_idx ON ext_test_table (id)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateIndex)
		assertObjectNames(t, stmt, "ext_test_idx", "ext_test_table")
	})
}

func TestParseStatement_AlterTable(t *testing.T) {
	t.Run("重命名表", func(t *testing.T) {
		stmt, err := ParseStatement("ALTER TABLE ext_test_table RENAME TO ext_new_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeAlterTable)
		assertObjectNames(t, stmt, "ext_test_table", "ext_new_table")
		assertObjectKinds(t, stmt, "table", "table")
	})

	t.Run("添加列", func(t *testing.T) {
		stmt, err := ParseStatement("ALTER TABLE ext_test_table ADD COLUMN new_col TEXT")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeAlterTable)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("不带 COLUMN 关键字添加列", func(t *testing.T) {
		stmt, err := ParseStatement("ALTER TABLE ext_test_table ADD another_col INTEGER DEFAULT 0")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeAlterTable)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("重命名列", func(t *testing.T) {
		stmt, err := ParseStatement("ALTER TABLE ext_test_table RENAME COLUMN old_col TO new_col")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeAlterTable)
		assertObjectNames(t, stmt, "ext_test_table")
	})
}

func TestParseStatement_DropStatements(t *testing.T) {
	t.Run("删除表", func(t *testing.T) {
		stmt, err := ParseStatement("DROP TABLE ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropTable)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("删除表 IF EXISTS", func(t *testing.T) {
		stmt, err := ParseStatement("DROP TABLE IF EXISTS ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropTable)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("删除视图", func(t *testing.T) {
		stmt, err := ParseStatement("DROP VIEW ext_test_view")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropView)
		assertObjectNames(t, stmt, "ext_test_view")
		assertObjectKinds(t, stmt, "view")
	})

	t.Run("删除视图 IF EXISTS", func(t *testing.T) {
		stmt, err := ParseStatement("DROP VIEW IF EXISTS ext_test_view")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropView)
		assertObjectNames(t, stmt, "ext_test_view")
	})

	t.Run("删除触发器", func(t *testing.T) {
		stmt, err := ParseStatement("DROP TRIGGER ext_test_trigger")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropTrigger)
		assertObjectNames(t, stmt, "ext_test_trigger")
		assertObjectKinds(t, stmt, "trigger")
	})

	t.Run("删除触发器 IF EXISTS", func(t *testing.T) {
		stmt, err := ParseStatement("DROP TRIGGER IF EXISTS ext_test_trigger")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropTrigger)
		assertObjectNames(t, stmt, "ext_test_trigger")
	})

	t.Run("删除索引", func(t *testing.T) {
		stmt, err := ParseStatement("DROP INDEX ext_test_idx")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropIndex)
		assertObjectNames(t, stmt, "ext_test_idx")
		assertObjectKinds(t, stmt, "index")
	})

	t.Run("删除索引 IF EXISTS", func(t *testing.T) {
		stmt, err := ParseStatement("DROP INDEX IF EXISTS ext_test_idx")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDropIndex)
		assertObjectNames(t, stmt, "ext_test_idx")
	})
}

func TestParseStatement_Insert(t *testing.T) {
	t.Run("普通插入语句", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table (id, name) VALUES (1, 'test')")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("INSERT OR REPLACE", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT OR REPLACE INTO ext_test_table (id) VALUES (1)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("INSERT OR IGNORE", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT OR IGNORE INTO ext_test_table (id) VALUES (1)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("INSERT INTO SELECT", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table SELECT * FROM ext_other_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
	})

	t.Run("不带列名插入", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table VALUES (1, 'test')")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table")
	})
}

func TestParseStatement_Update(t *testing.T) {
	t.Run("普通更新语句", func(t *testing.T) {
		stmt, err := ParseStatement("UPDATE ext_test_table SET name = 'test' WHERE id = 1")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeUpdate)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("UPDATE OR IGNORE", func(t *testing.T) {
		stmt, err := ParseStatement("UPDATE OR IGNORE ext_test_table SET name = 'test'")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeUpdate)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("UPDATE FROM 多表", func(t *testing.T) {
		stmt, err := ParseStatement("UPDATE ext_test_table SET name = 'test' FROM ext_other_table WHERE ext_test_table.id = ext_other_table.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeUpdate)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
	})

	t.Run("无 WHERE 更新语句", func(t *testing.T) {
		stmt, err := ParseStatement("UPDATE ext_test_table SET count = 0")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeUpdate)
		assertObjectNames(t, stmt, "ext_test_table")
	})
}

func TestParseStatement_Delete(t *testing.T) {
	t.Run("普通删除语句", func(t *testing.T) {
		stmt, err := ParseStatement("DELETE FROM ext_test_table WHERE id = 1")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDelete)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("无 WHERE 删除语句", func(t *testing.T) {
		stmt, err := ParseStatement("DELETE FROM ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDelete)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("带 USING 的删除语句", func(t *testing.T) {
		stmt, err := ParseStatement("DELETE FROM ext_test_table USING ext_other_table WHERE ext_test_table.id = ext_other_table.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeDelete)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
	})
}

func TestParseStatement_Select(t *testing.T) {
	t.Run("简单查询", func(t *testing.T) {
		stmt, err := ParseStatement("SELECT * FROM ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_test_table")
		assertObjectKinds(t, stmt, "table")
	})

	t.Run("多表 JOIN 查询", func(t *testing.T) {
		stmt, err := ParseStatement("SELECT * FROM ext_test_table t1 JOIN ext_other_table t2 ON t1.id = t2.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
		assertObjectKinds(t, stmt, "table", "table")
	})

	t.Run("LEFT JOIN 查询", func(t *testing.T) {
		stmt, err := ParseStatement("SELECT * FROM ext_test_table LEFT JOIN ext_other_table ON ext_test_table.id = ext_other_table.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
	})

	t.Run("三表 JOIN 查询", func(t *testing.T) {
		stmt, err := ParseStatement("SELECT * FROM ext_table_a a INNER JOIN ext_table_b b ON a.id = b.id INNER JOIN ext_table_c c ON b.id = c.id")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_table_a", "ext_table_b", "ext_table_c")
	})

	t.Run("子查询", func(t *testing.T) {
		stmt, err := ParseStatement("SELECT * FROM ext_test_table WHERE id IN (SELECT id FROM ext_other_table)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_test_table", "ext_other_table")
	})

	t.Run("WITH 子句查询", func(t *testing.T) {
		stmt, err := ParseStatement("WITH cte AS (SELECT id FROM ext_test_table) SELECT * FROM cte")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeSelect)
		assertObjectNames(t, stmt, "ext_test_table")
	})
}

func TestParseStatement_StringWithSemicolon(t *testing.T) {
	t.Run("字符串内包含分号", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table VALUES ('hello;world')")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
		assertObjectNames(t, stmt, "ext_test_table")
	})

	t.Run("字符串内包含多个分号", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table VALUES ('a;b;c;d')")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
	})

	t.Run("字符串内包含转义单引号和分号", func(t *testing.T) {
		stmt, err := ParseStatement("INSERT INTO ext_test_table VALUES ('it''s;a;test')")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeInsert)
	})
}

func TestParseStatement_QuotedIdentifiers(t *testing.T) {
	t.Run("双引号表名", func(t *testing.T) {
		stmt, err := ParseStatement(`CREATE TABLE "ext_table" (id INTEGER)`)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_table")
	})

	t.Run("方括号表名", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TABLE [ext_table] (id INTEGER)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_table")
	})

	t.Run("反引号表名", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TABLE `ext_table` (id INTEGER)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext_table")
	})

	t.Run("双引号内转义双引号", func(t *testing.T) {
		stmt, err := ParseStatement(`CREATE TABLE "ext""table" (id INTEGER)`)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, `ext"table`)
	})

	t.Run("双引号带特殊字符表名", func(t *testing.T) {
		stmt, err := ParseStatement(`CREATE TABLE "ext table" (id INTEGER)`)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
		assertObjectNames(t, stmt, "ext table")
	})

	t.Run("带模式名的表名", func(t *testing.T) {
		stmt, err := ParseStatement("CREATE TABLE main.ext_test_table (id INTEGER)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeCreateTable)
	})
}

func TestParseStatement_Unparseable(t *testing.T) {
	t.Run("空字符串", func(t *testing.T) {
		_, err := ParseStatement("")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("纯空白字符串", func(t *testing.T) {
		_, err := ParseStatement("   \n\t  ")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("数字开头的非法SQL", func(t *testing.T) {
		_, err := ParseStatement("123abc")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("标点开头的非法SQL", func(t *testing.T) {
		_, err := ParseStatement("(SELECT 1)")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("不支持的CREATE类型", func(t *testing.T) {
		_, err := ParseStatement("CREATE FOO bar")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("不支持的DROP类型", func(t *testing.T) {
		_, err := ParseStatement("DROP FOO bar")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("ALTER 缺少 TABLE", func(t *testing.T) {
		_, err := ParseStatement("ALTER INDEX ext_test_idx")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("INSERT 缺少 INTO", func(t *testing.T) {
		_, err := ParseStatement("INSERT ext_test_table VALUES (1)")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("DELETE 缺少 FROM", func(t *testing.T) {
		_, err := ParseStatement("DELETE ext_test_table")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("CREATE TABLE 缺少表名", func(t *testing.T) {
		_, err := ParseStatement("CREATE TABLE (id INTEGER)")
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})
}

func TestParseStatement_Other(t *testing.T) {
	t.Run("PRAGMA 语句", func(t *testing.T) {
		stmt, err := ParseStatement("PRAGMA table_info(ext_test_table)")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})

	t.Run("ANALYZE 语句", func(t *testing.T) {
		stmt, err := ParseStatement("ANALYZE ext_test_table")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})

	t.Run("VACUUM 语句", func(t *testing.T) {
		stmt, err := ParseStatement("VACUUM")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})

	t.Run("BEGIN TRANSACTION 语句", func(t *testing.T) {
		stmt, err := ParseStatement("BEGIN TRANSACTION")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})

	t.Run("COMMIT 语句", func(t *testing.T) {
		stmt, err := ParseStatement("COMMIT")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})

	t.Run("ATTACH DATABASE 语句", func(t *testing.T) {
		stmt, err := ParseStatement("ATTACH DATABASE 'test.db' AS test_db")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		assertStmtType(t, stmt, SQLTypeOther)
	})
}

func TestSplitSQLStatements(t *testing.T) {
	t.Run("单条语句无分号", func(t *testing.T) {
		stmts := splitSQLStatements("CREATE TABLE ext_test_table (id INTEGER)")
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("单条语句带分号", func(t *testing.T) {
		stmts := splitSQLStatements("CREATE TABLE ext_test_table (id INTEGER);")
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("多条语句用分号分隔", func(t *testing.T) {
		raw := "CREATE TABLE ext_a (id INTEGER);\nCREATE TABLE ext_b (id INTEGER);"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 2 {
			t.Fatalf("期望 2 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("字符串内分号不分割", func(t *testing.T) {
		raw := "INSERT INTO ext_test_table VALUES ('hello;world');"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})

	t.Run("双引号内分号不分割", func(t *testing.T) {
		raw := `CREATE TABLE "ext;table" (id INTEGER);`
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})

	t.Run("方括号内分号不分割", func(t *testing.T) {
		raw := "CREATE TABLE [ext;table] (id INTEGER);"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})

	t.Run("行注释不产生语句", func(t *testing.T) {
		raw := "CREATE TABLE ext_a (id INTEGER);\n-- this is a comment\n"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("块注释不产生语句", func(t *testing.T) {
		raw := "CREATE TABLE ext_a (id INTEGER);\n/* block comment */\n"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("块注释在语句前不丢弃语句", func(t *testing.T) {
		raw := "/* block comment */\nCREATE TABLE ext_a (id INTEGER);"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("空字符串返回空切片", func(t *testing.T) {
		stmts := splitSQLStatements("")
		if len(stmts) != 0 {
			t.Fatalf("期望 0 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("纯注释返回空切片", func(t *testing.T) {
		stmts := splitSQLStatements("-- only comment\n")
		if len(stmts) != 0 {
			t.Fatalf("期望 0 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("BEGIN END 块内分号不分割", func(t *testing.T) {
		raw := `CREATE TRIGGER ext_trig BEFORE INSERT ON ext_test_table
BEGIN
    INSERT INTO ext_log VALUES (1);
    UPDATE ext_log SET msg = 'test';
END;`
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})

	t.Run("CASE END 不影响语句分割", func(t *testing.T) {
		raw := "SELECT CASE WHEN id = 1 THEN 'a' ELSE 'b' END FROM ext_test_table;"
		stmts := splitSQLStatements(raw)
		if len(stmts) != 1 {
			t.Fatalf("期望 1 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})

	t.Run("多个触发器语句", func(t *testing.T) {
		raw := `CREATE TRIGGER ext_trig1 BEFORE INSERT ON ext_test_table BEGIN END;
CREATE TRIGGER ext_trig2 AFTER DELETE ON ext_test_table BEGIN END;`
		stmts := splitSQLStatements(raw)
		if len(stmts) != 2 {
			t.Fatalf("期望 2 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("混合语句分割", func(t *testing.T) {
		raw := `CREATE TABLE ext_a (id INTEGER);
INSERT INTO ext_a VALUES (1);
CREATE TRIGGER ext_trig BEFORE INSERT ON ext_a BEGIN
    UPDATE ext_a SET id = id + 1;
END;
DROP TABLE ext_a;`
		stmts := splitSQLStatements(raw)
		if len(stmts) != 4 {
			t.Fatalf("期望 4 条语句，实际 %d（%v）", len(stmts), stmts)
		}
	})
}

func TestParseStatements(t *testing.T) {
	t.Run("多语句解析", func(t *testing.T) {
		raw := `CREATE TABLE ext_a (id INTEGER);
INSERT INTO ext_a VALUES (1);
SELECT * FROM ext_a;`
		stmts, err := ParseStatements(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 3 {
			t.Fatalf("期望 3 条语句，实际 %d", len(stmts))
		}
		assertStmtType(t, stmts[0], SQLTypeCreateTable)
		assertStmtType(t, stmts[1], SQLTypeInsert)
		assertStmtType(t, stmts[2], SQLTypeSelect)
	})

	t.Run("包含块注释的多语句解析", func(t *testing.T) {
		raw := `/* 创建表 */
CREATE TABLE ext_a (id INTEGER);
/* 创建索引 */
CREATE INDEX ext_idx ON ext_a (id);`
		stmts, err := ParseStatements(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 2 {
			t.Fatalf("期望 2 条语句，实际 %d", len(stmts))
		}
		assertStmtType(t, stmts[0], SQLTypeCreateTable)
		assertStmtType(t, stmts[1], SQLTypeCreateIndex)
	})

	t.Run("包含触发器的多语句解析", func(t *testing.T) {
		raw := `CREATE TABLE ext_test_table (id INTEGER);
CREATE TRIGGER ext_trig BEFORE INSERT ON ext_test_table
BEGIN
    INSERT INTO ext_log VALUES (1);
END;
CREATE TABLE ext_log (id INTEGER);`
		stmts, err := ParseStatements(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 3 {
			t.Fatalf("期望 3 条语句，实际 %d", len(stmts))
		}
		assertStmtType(t, stmts[0], SQLTypeCreateTable)
		assertStmtType(t, stmts[1], SQLTypeCreateTrigger)
		assertStmtType(t, stmts[2], SQLTypeCreateTable)
	})

	t.Run("空字符串返回空切片", func(t *testing.T) {
		stmts, err := ParseStatements("")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 0 {
			t.Fatalf("期望 0 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("纯注释返回空切片", func(t *testing.T) {
		stmts, err := ParseStatements("-- only comment\n/* block */")
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 0 {
			t.Fatalf("期望 0 条语句，实际 %d", len(stmts))
		}
	})

	t.Run("错误语句传播", func(t *testing.T) {
		raw := "CREATE TABLE ext_a (id INTEGER);\nCREATE FOO bar;"
		_, err := ParseStatements(raw)
		if !errors.Is(err, ErrSQLUnparseable) {
			t.Errorf("期望 ErrSQLUnparseable，实际 %v", err)
		}
	})

	t.Run("完整DDL脚本解析", func(t *testing.T) {
		raw := `CREATE TABLE IF NOT EXISTS ext_users (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE IF NOT EXISTS ext_orders (id INTEGER PRIMARY KEY, user_id INTEGER);
CREATE INDEX IF NOT EXISTS ext_idx_orders_user ON ext_orders (user_id);
CREATE VIEW ext_user_orders AS SELECT u.name, o.id FROM ext_users u JOIN ext_orders o ON u.id = o.user_id;
ALTER TABLE ext_users ADD COLUMN email TEXT;
DROP INDEX IF EXISTS ext_old_idx;
DROP TABLE IF EXISTS ext_old_table;`
		stmts, err := ParseStatements(raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if len(stmts) != 7 {
			t.Fatalf("期望 7 条语句，实际 %d", len(stmts))
		}
		assertStmtType(t, stmts[0], SQLTypeCreateTable)
		assertStmtType(t, stmts[1], SQLTypeCreateTable)
		assertStmtType(t, stmts[2], SQLTypeCreateIndex)
		assertStmtType(t, stmts[3], SQLTypeCreateView)
		assertStmtType(t, stmts[4], SQLTypeAlterTable)
		assertStmtType(t, stmts[5], SQLTypeDropIndex)
		assertStmtType(t, stmts[6], SQLTypeDropTable)
	})
}

func TestTokenizeSQL(t *testing.T) {
	t.Run("单词和标点", func(t *testing.T) {
		tokens := tokenizeSQL("CREATE TABLE ext_a")
		if len(tokens) < 4 {
			t.Fatalf("期望至少 4 个 token，实际 %d", len(tokens))
		}
		if tokens[0].kind != tokWord || tokens[0].value != "CREATE" {
			t.Errorf("第 0 个 token 期望 WORD/CREATE，实际 %v/%s", tokens[0].kind, tokens[0].value)
		}
		if tokens[1].kind != tokWord || tokens[1].value != "TABLE" {
			t.Errorf("第 1 个 token 期望 WORD/TABLE，实际 %v/%s", tokens[1].kind, tokens[1].value)
		}
		if tokens[2].kind != tokWord || tokens[2].value != "ext_a" {
			t.Errorf("第 2 个 token 期望 WORD/ext_a，实际 %v/%s", tokens[2].kind, tokens[2].value)
		}
		if tokens[3].kind != tokEOF {
			t.Errorf("第 3 个 token 期望 EOF，实际 %v", tokens[3].kind)
		}
	})

	t.Run("字符串 token", func(t *testing.T) {
		tokens := tokenizeSQL("'hello world'")
		if tokens[0].kind != tokString {
			t.Errorf("期望 STRING，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != "'hello world'" {
			t.Errorf("期望值 'hello world'，实际 %s", tokens[0].value)
		}
	})

	t.Run("转义单引号字符串", func(t *testing.T) {
		tokens := tokenizeSQL("'it''s ok'")
		if tokens[0].kind != tokString {
			t.Errorf("期望 STRING，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != "'it''s ok'" {
			t.Errorf("期望值 'it''s ok'，实际 %s", tokens[0].value)
		}
	})

	t.Run("数字 token", func(t *testing.T) {
		tokens := tokenizeSQL("123 45.67")
		if tokens[0].kind != tokNumber || tokens[0].value != "123" {
			t.Errorf("第 0 个 token 期望 NUMBER/123，实际 %v/%s", tokens[0].kind, tokens[0].value)
		}
		if tokens[1].kind != tokNumber || tokens[1].value != "45.67" {
			t.Errorf("第 1 个 token 期望 NUMBER/45.67，实际 %v/%s", tokens[1].kind, tokens[1].value)
		}
	})

	t.Run("双引号标识符", func(t *testing.T) {
		tokens := tokenizeSQL(`"ext_table"`)
		if tokens[0].kind != tokIdent {
			t.Errorf("期望 IDENT，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != "ext_table" {
			t.Errorf("期望值 ext_table，实际 %s", tokens[0].value)
		}
	})

	t.Run("方括号标识符", func(t *testing.T) {
		tokens := tokenizeSQL("[ext_table]")
		if tokens[0].kind != tokIdent {
			t.Errorf("期望 IDENT，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != "ext_table" {
			t.Errorf("期望值 ext_table，实际 %s", tokens[0].value)
		}
	})

	t.Run("反引号标识符", func(t *testing.T) {
		tokens := tokenizeSQL("`ext_table`")
		if tokens[0].kind != tokIdent {
			t.Errorf("期望 IDENT，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != "ext_table" {
			t.Errorf("期望值 ext_table，实际 %s", tokens[0].value)
		}
	})

	t.Run("双引号内转义双引号", func(t *testing.T) {
		tokens := tokenizeSQL(`"ext""table"`)
		if tokens[0].kind != tokIdent {
			t.Errorf("期望 IDENT，实际 %v", tokens[0].kind)
		}
		if tokens[0].value != `ext"table` {
			t.Errorf(`期望值 ext"table，实际 %s`, tokens[0].value)
		}
	})

	t.Run("行注释被跳过", func(t *testing.T) {
		tokens := tokenizeSQL("CREATE -- comment\nTABLE")
		if len(tokens) < 3 {
			t.Fatalf("期望至少 3 个 token，实际 %d", len(tokens))
		}
		if tokens[0].value != "CREATE" {
			t.Errorf("第 0 个 token 期望 CREATE，实际 %s", tokens[0].value)
		}
		if tokens[1].value != "TABLE" {
			t.Errorf("第 1 个 token 期望 TABLE，实际 %s", tokens[1].value)
		}
	})

	t.Run("块注释被跳过", func(t *testing.T) {
		tokens := tokenizeSQL("CREATE /* comment */ TABLE")
		if len(tokens) < 3 {
			t.Fatalf("期望至少 3 个 token，实际 %d", len(tokens))
		}
		if tokens[0].value != "CREATE" {
			t.Errorf("第 0 个 token 期望 CREATE，实际 %s", tokens[0].value)
		}
		if tokens[1].value != "TABLE" {
			t.Errorf("第 1 个 token 期望 TABLE，实际 %s", tokens[1].value)
		}
	})

	t.Run("标点 token", func(t *testing.T) {
		tokens := tokenizeSQL("()")
		if tokens[0].kind != tokPunct || tokens[0].value != "(" {
			t.Errorf("第 0 个 token 期望 PUNCT/(，实际 %v/%s", tokens[0].kind, tokens[0].value)
		}
		if tokens[1].kind != tokPunct || tokens[1].value != ")" {
			t.Errorf("第 1 个 token 期望 PUNCT/)，实际 %v/%s", tokens[1].kind, tokens[1].value)
		}
	})

	t.Run("空字符串返回 EOF", func(t *testing.T) {
		tokens := tokenizeSQL("")
		if len(tokens) != 1 {
			t.Fatalf("期望 1 个 token，实际 %d", len(tokens))
		}
		if tokens[0].kind != tokEOF {
			t.Errorf("期望 EOF，实际 %v", tokens[0].kind)
		}
	})

	t.Run("混合 token 序列", func(t *testing.T) {
		tokens := tokenizeSQL("INSERT INTO ext_t (id) VALUES (1, 'a')")
		if tokens[0].kind != tokWord || tokens[0].value != "INSERT" {
			t.Errorf("第 0 个 token 期望 WORD/INSERT，实际 %v/%s", tokens[0].kind, tokens[0].value)
		}
		if tokens[3].kind != tokPunct || tokens[3].value != "(" {
			t.Errorf("第 3 个 token 期望 PUNCT/(，实际 %v/%s", tokens[3].kind, tokens[3].value)
		}
		if tokens[4].kind != tokWord || tokens[4].value != "id" {
			t.Errorf("第 4 个 token 期望 WORD/id，实际 %v/%s", tokens[4].kind, tokens[4].value)
		}
		if tokens[8].kind != tokNumber || tokens[8].value != "1" {
			t.Errorf("第 8 个 token 期望 NUMBER/1，实际 %v/%s", tokens[8].kind, tokens[8].value)
		}
		if tokens[10].kind != tokString || tokens[10].value != "'a'" {
			t.Errorf("第 10 个 token 期望 STRING/'a'，实际 %v/%s", tokens[10].kind, tokens[10].value)
		}
	})
}

func TestSQLStatement_AllObjectNames(t *testing.T) {
	t.Run("提取对象名称列表", func(t *testing.T) {
		stmt := &SQLStatement{
			Objects: []SQLObjectRef{
				{Name: "ext_table", Kind: "table"},
				{Name: "ext_idx", Kind: "index"},
			},
		}
		names := stmt.AllObjectNames()
		if len(names) != 2 {
			t.Fatalf("期望 2 个名称，实际 %d", len(names))
		}
		if names[0] != "ext_table" {
			t.Errorf("第 0 个名称期望 ext_table，实际 %s", names[0])
		}
		if names[1] != "ext_idx" {
			t.Errorf("第 1 个名称期望 ext_idx，实际 %s", names[1])
		}
	})

	t.Run("空对象列表", func(t *testing.T) {
		stmt := &SQLStatement{Objects: nil}
		names := stmt.AllObjectNames()
		if len(names) != 0 {
			t.Fatalf("期望 0 个名称，实际 %d", len(names))
		}
	})
}
