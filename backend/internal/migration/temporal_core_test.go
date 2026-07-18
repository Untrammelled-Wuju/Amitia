package migration

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemporalCoreMigrationCreatesCanonicalTables(t *testing.T){
	db,err:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{})
	if err!=nil{t.Fatal(err)}
	runner:=Runner{DB:db,SkipBackup:true}
	if err:=runner.Apply([]Migration{TemporalCoreMigration()});err!=nil{t.Fatal(err)}
	for _,table:=range []string{"temporal_profiles","temporal_anchors","temporal_events","memory_temporal_metadata"}{if !db.Migrator().HasTable(table){t.Fatalf("missing table %s",table)}}
	if db.Migrator().HasTable("temporal_presence_states"){t.Fatal("Track T must not create presence tables")}
}
