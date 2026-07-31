package migration

import (
	"strings"
	"testing"
)

func TestNormalizeExtensionID(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"斜杠分隔符替换为下划线", "com.example/weather", "com_example_weather"},
		{"点号分隔符替换为下划线", "com.example.weather", "com_example_weather"},
		{"短扩展名", "ext.drift", "ext_drift"},
		{"空字符串返回unknown", "", "unknown"},
		{"仅包含特殊字符返回unknown", "...///", "unknown"},
		{"大写字母转小写", "COM.Example/Weather", "com_example_weather"},
		{"包含数字保持不变", "com.example.v2/weather", "com_example_v2_weather"},
		{"连字符替换为下划线", "com-example-weather", "com_example_weather"},
		{"首尾特殊字符被裁剪", ".com.example.", "com_example"},
		{"纯字母数字不变", "abc123", "abc123"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeExtensionID(c.input)
			if got != c.want {
				t.Errorf("NormalizeExtensionID(%q) = %q, 期望 %q", c.input, got, c.want)
			}
		})
	}
}

func TestExtensionNamespacePrefix(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		want string
	}{
		{"com.example/weather", "com.example/weather", "ext_com_example_weather_"},
		{"com.example.weather", "com.example.weather", "ext_com_example_weather_"},
		{"ext.drift", "ext.drift", "ext_ext_drift_"},
		{"空字符串", "", "ext_unknown_"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtensionNamespacePrefix(c.ext)
			if got != c.want {
				t.Errorf("ExtensionNamespacePrefix(%q) = %q, 期望 %q", c.ext, got, c.want)
			}
		})
	}
}

func TestIsSystemTable(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"sqlite_master", "sqlite_master", true},
		{"sqlite_sequence", "sqlite_sequence", true},
		{"sqlite_stat1", "sqlite_stat1", true},
		{"sqlite_stat2前缀匹配", "sqlite_stat2", true},
		{"sqlite_schema", "sqlite_schema", true},
		{"sqlite_temp_master", "sqlite_temp_master", true},
		{"ext_table非系统表", "ext_table", false},
		{"users非系统表", "users", false},
		{"大写SQLITE_MASTER", "SQLITE_MASTER", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsSystemTable(c.input)
			if got != c.want {
				t.Errorf("IsSystemTable(%q) = %v, 期望 %v", c.input, got, c.want)
			}
		})
	}
}

func TestIsHostTable(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"users是宿主表", "users", true},
		{"messages是宿主表", "messages", true},
		{"characters是宿主表", "characters", true},
		{"extension_kv_state是宿主表", "extension_kv_state", true},
		{"extension_definitions是宿主表", "extension_definitions", true},
		{"schema_migrations是宿主表", "schema_migrations", true},
		{"ext_table非宿主表", "ext_table", false},
		{"大写USERS", "USERS", true},
		{"随机表名非宿主表", "random_table", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsHostTable(c.input)
			if got != c.want {
				t.Errorf("IsHostTable(%q) = %v, 期望 %v", c.input, got, c.want)
			}
		})
	}
}

func TestValidateStatement(t *testing.T) {
	t.Run("nil语句返回错误", func(t *testing.T) {
		err := ValidateStatement(nil, "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "nil") {
			t.Errorf("错误信息应包含 nil, 实际: %v", err)
		}
	})

	cases := []struct {
		name      string
		raw       string
		extID     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "扩展A访问扩展B的表应失败",
			raw:       "SELECT * FROM ext_other_extension_cache",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "does not belong to namespace",
		},
		{
			name:      "访问宿主表应失败",
			raw:       "SELECT * FROM users",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "host table",
		},
		{
			name:      "访问系统表应失败",
			raw:       "SELECT * FROM sqlite_master",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "system table",
		},
		{
			name:    "合法namespace表应成功",
			raw:     "CREATE TABLE ext_com_example_weather_cache (id INTEGER)",
			extID:   "com.example.weather",
			wantErr: false,
		},
		{
			name:      "禁止ATTACH应失败",
			raw:       "ATTACH DATABASE 'other.db' AS other",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "禁止DETACH应失败",
			raw:       "DETACH DATABASE other",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "禁止load_extension应失败",
			raw:       "SELECT load_extension('malicious.dll')",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "禁止PRAGMA writable_schema应失败",
			raw:       "PRAGMA writable_schema = 1",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "禁止VACUUM应失败",
			raw:       "VACUUM",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "禁止PRAGMA foreign_keys应失败",
			raw:       "PRAGMA foreign_keys = ON",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "不属于namespace的ext_表应失败",
			raw:       "CREATE TABLE ext_other_table (id INTEGER)",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "does not belong to namespace",
		},
		{
			name:      "CREATE TRIGGER ON宿主表应失败",
			raw:       "CREATE TRIGGER ext_com_example_weather_trg AFTER INSERT ON users BEGIN SELECT 1; END",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "CREATE TRIGGER ON namespace表应失败_forbidden",
			raw:       "CREATE TRIGGER ext_com_example_weather_trg AFTER INSERT ON ext_com_example_weather_cache BEGIN SELECT 1; END",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "forbidden command",
		},
		{
			name:      "CREATE INDEX ON宿主表应失败",
			raw:       "CREATE INDEX ext_com_example_weather_idx ON users (id)",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "host table",
		},
		{
			name:    "CREATE INDEX ON namespace表应成功",
			raw:     "CREATE INDEX ext_com_example_weather_idx ON ext_com_example_weather_cache (id)",
			extID:   "com.example.weather",
			wantErr: false,
		},
		{
			name:    "ALTER TABLE namespace表应成功",
			raw:     "ALTER TABLE ext_com_example_weather_cache ADD COLUMN name TEXT",
			extID:   "com.example.weather",
			wantErr: false,
		},
		{
			name:      "ALTER TABLE宿主表应失败",
			raw:       "ALTER TABLE users ADD COLUMN name TEXT",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "host table",
		},
		{
			name:    "DROP TABLE namespace表应成功",
			raw:     "DROP TABLE IF EXISTS ext_com_example_weather_cache",
			extID:   "com.example.weather",
			wantErr: false,
		},
		{
			name:    "INSERT INTO namespace表应成功",
			raw:     "INSERT INTO ext_com_example_weather_cache (id) VALUES (1)",
			extID:   "com.example.weather",
			wantErr: false,
		},
		{
			name:      "无ext_前缀的表应失败",
			raw:       "CREATE TABLE random_table (id INTEGER)",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "ext_ prefix",
		},
		{
			name:      "SQLTypeOther且无禁止命令应拒绝",
			raw:       "REINDEX ext_com_example_weather_idx",
			extID:     "com.example.weather",
			wantErr:   true,
			errSubstr: "unparseable",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stmt, err := ParseStatement(c.raw)
			if err != nil {
				t.Fatalf("解析SQL失败 %q: %v", c.raw, err)
			}
			err = ValidateStatement(stmt, c.extID)
			if c.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误, 实际为nil")
				}
				if c.errSubstr != "" && !strings.Contains(err.Error(), c.errSubstr) {
					t.Errorf("错误信息应包含 %q, 实际: %v", c.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("期望无错误, 实际: %v", err)
				}
			}
		})
	}
}

func TestValidateRawStatements(t *testing.T) {
	t.Run("空SQL应返回错误", func(t *testing.T) {
		_, err := ValidateRawStatements("", "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "no executable statements") {
			t.Errorf("错误信息应包含 no executable statements, 实际: %v", err)
		}
	})

	t.Run("仅空白字符应返回错误", func(t *testing.T) {
		_, err := ValidateRawStatements("   \n\t  ", "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "no executable statements") {
			t.Errorf("错误信息应包含 no executable statements, 实际: %v", err)
		}
	})

	t.Run("仅注释应返回错误", func(t *testing.T) {
		_, err := ValidateRawStatements("-- this is a comment", "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "no executable statements") {
			t.Errorf("错误信息应包含 no executable statements, 实际: %v", err)
		}
	})

	t.Run("合法多语句SQL应成功", func(t *testing.T) {
		sql := `CREATE TABLE ext_com_example_weather_cache (id INTEGER);
CREATE INDEX ext_com_example_weather_idx ON ext_com_example_weather_cache (id);
INSERT INTO ext_com_example_weather_cache (id) VALUES (1);`
		stmts, err := ValidateRawStatements(sql, "com.example.weather")
		if err != nil {
			t.Fatalf("期望无错误, 实际: %v", err)
		}
		if len(stmts) != 3 {
			t.Errorf("期望解析3条语句, 实际 %d", len(stmts))
		}
	})

	t.Run("多语句中包含非法语句应失败", func(t *testing.T) {
		sql := `CREATE TABLE ext_com_example_weather_cache (id INTEGER);
SELECT * FROM users;
CREATE INDEX ext_com_example_weather_idx ON ext_com_example_weather_cache (id);`
		_, err := ValidateRawStatements(sql, "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "host table") {
			t.Errorf("错误信息应包含 host table, 实际: %v", err)
		}
	})

	t.Run("多语句中包含ATTACH应失败", func(t *testing.T) {
		sql := `CREATE TABLE ext_com_example_weather_cache (id INTEGER);
ATTACH DATABASE 'other.db' AS other;`
		_, err := ValidateRawStatements(sql, "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "forbidden command") {
			t.Errorf("错误信息应包含 forbidden command, 实际: %v", err)
		}
	})

	t.Run("包含触发器的多语句应失败", func(t *testing.T) {
		sql := `CREATE TABLE ext_com_example_weather_cache (id INTEGER, name TEXT);
CREATE TRIGGER ext_com_example_weather_trg AFTER INSERT ON ext_com_example_weather_cache
BEGIN
    UPDATE ext_com_example_weather_cache SET name = 'default' WHERE name IS NULL;
END;
CREATE INDEX ext_com_example_weather_idx ON ext_com_example_weather_cache (id);`
		_, err := ValidateRawStatements(sql, "com.example.weather")
		if err == nil {
			t.Fatal("期望返回错误, 实际为nil")
		}
		if !strings.Contains(err.Error(), "forbidden command") {
			t.Errorf("错误信息应包含 forbidden command, 实际: %v", err)
		}
	})

	t.Run("单条合法语句应成功", func(t *testing.T) {
		stmts, err := ValidateRawStatements(
			"CREATE TABLE ext_com_example_weather_cache (id INTEGER);",
			"com.example.weather",
		)
		if err != nil {
			t.Fatalf("期望无错误, 实际: %v", err)
		}
		if len(stmts) != 1 {
			t.Errorf("期望解析1条语句, 实际 %d", len(stmts))
		}
	})
}

func TestIsExtensionNamespaceTable(t *testing.T) {
	cases := []struct {
		name  string
		table string
		extID string
		want  bool
	}{
		{"namespace表返回true", "ext_com_example_weather_cache", "com.example.weather", true},
		{"namespace索引返回true", "ext_com_example_weather_idx", "com.example.weather", true},
		{"宿主表返回false", "users", "com.example.weather", false},
		{"系统表返回false", "sqlite_master", "com.example.weather", false},
		{"系统表sqlite_sequence返回false", "sqlite_sequence", "com.example.weather", false},
		{"其他扩展的ext_表返回false", "ext_other_extension_cache", "com.example.weather", false},
		{"无ext_前缀返回false", "random_table", "com.example.weather", false},
		{"不同扩展名匹配", "ext_ext_drift_cache", "ext.drift", true},
		{"空表名返回false", "", "com.example.weather", false},
		{"大写表名匹配", "EXT_COM_EXAMPLE_WEATHER_CACHE", "com.example.weather", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsExtensionNamespaceTable(c.table, c.extID)
			if got != c.want {
				t.Errorf("IsExtensionNamespaceTable(%q, %q) = %v, 期望 %v", c.table, c.extID, got, c.want)
			}
		})
	}
}
