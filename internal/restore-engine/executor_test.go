package restoreengine

import (
	"testing"
)

func TestDropGuardStmt_BasicProcedure(t *testing.T) {
	tests := []struct {
		name string
		stmt string
		want string // expected DROP IF EXISTS, empty string means nil
	}{
		{
			name: "simple CREATE PROCEDURE",
			stmt: "CREATE PROCEDURE get_products_by_category() BEGIN SELECT 1; END",
			want: "DROP PROCEDURE IF EXISTS `get_products_by_category`",
		},
		{
			name: "CREATE PROCEDURE with DEFINER",
			stmt: "CREATE DEFINER=`admin`@`%` PROCEDURE `my_proc`() BEGIN SELECT 1; END",
			want: "DROP PROCEDURE IF EXISTS `my_proc`",
		},
		{
			name: "CREATE PROCEDURE with DEFINER unquoted",
			stmt: "CREATE DEFINER=admin@% PROCEDURE my_proc() BEGIN SELECT 1; END",
			want: "DROP PROCEDURE IF EXISTS `my_proc`",
		},
		{
			name: "CREATE FUNCTION",
			stmt: "CREATE FUNCTION my_func() RETURNS INT BEGIN RETURN 1; END",
			want: "DROP FUNCTION IF EXISTS `my_func`",
		},
		{
			name: "CREATE TRIGGER",
			stmt: "CREATE TRIGGER my_trig BEFORE INSERT ON t FOR EACH ROW BEGIN END",
			want: "DROP TRIGGER IF EXISTS `my_trig`",
		},
		{
			name: "CREATE EVENT",
			stmt: "CREATE EVENT my_event ON SCHEDULE EVERY 1 DAY DO BEGIN END",
			want: "DROP EVENT IF EXISTS `my_event`",
		},
		{
			name: "CREATE TABLE (not a routine)",
			stmt: "CREATE TABLE t (id INT)",
			want: "", // nil — no DROP guard for tables
		},
		{
			name: "CREATE OR REPLACE PROCEDURE",
			stmt: "CREATE OR REPLACE PROCEDURE my_proc() BEGIN END",
			want: "", // nil — OR REPLACE already handles it
		},
		{
			name: "CREATE PROCEDURE IF NOT EXISTS",
			stmt: "CREATE PROCEDURE IF NOT EXISTS my_proc() BEGIN END",
			want: "", // nil — IF NOT EXISTS already handles it
		},
		{
			name: "INSERT statement",
			stmt: "INSERT INTO t VALUES (1)",
			want: "", // nil — not a CREATE
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dropGuardStmt([]byte(tt.stmt))
			if tt.want == "" {
				if got != nil {
					t.Errorf("dropGuardStmt() = %q, want nil", string(got))
				}
				return
			}
			if got == nil {
				t.Errorf("dropGuardStmt() = nil, want %q", tt.want)
				return
			}
			if string(got) != tt.want {
				t.Errorf("dropGuardStmt() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestDropGuardStmt_VersionComments(t *testing.T) {
	tests := []struct {
		name string
		stmt string
		want string
	}{
		{
			name: "mysqldump CREATE PROCEDURE with version comments",
			stmt: "/*!50003 CREATE*/ /*!50020 DEFINER=`admin`@`%`*/ /*!50003 PROCEDURE `get_products_by_category`(IN category_id INT)\nBEGIN\n  SELECT * FROM products WHERE category = category_id;\nEND",
			want: "DROP PROCEDURE IF EXISTS `get_products_by_category`",
		},
		{
			name: "mysqldump CREATE FUNCTION with version comments",
			stmt: "/*!50003 CREATE*/ /*!50020 DEFINER=`admin`@`%`*/ /*!50003 FUNCTION `my_func`() RETURNS INT\nBEGIN\n  RETURN 1;\nEND",
			want: "DROP FUNCTION IF EXISTS `my_func`",
		},
		{
			name: "mysqldump CREATE TRIGGER with version comments",
			stmt: "/*!50003 CREATE*/ /*!50020 DEFINER=`admin`@`%`*/ /*!50003 TRIGGER `my_trig` BEFORE INSERT ON `t`\nFOR EACH ROW\nBEGIN\nEND",
			want: "DROP TRIGGER IF EXISTS `my_trig`",
		},
		{
			name: "mysqldump CREATE EVENT with version comments",
			stmt: "/*!50003 CREATE*/ /*!50020 DEFINER=`admin`@`%`*/ /*!50003 EVENT `my_event` ON SCHEDULE EVERY 1 DAY\nDO\nBEGIN\nEND",
			want: "DROP EVENT IF EXISTS `my_event`",
		},
		{
			name: "CREATE PROCEDURE with /*M! version comment",
			stmt: "/*M!100103 CREATE*/ /*!50020 DEFINER=`admin`@`%`*/ /*!50003 PROCEDURE `mariadb_proc`() BEGIN END",
			want: "DROP PROCEDURE IF EXISTS `mariadb_proc`",
		},
		{
			name: "version comment but no CREATE (should not match)",
			stmt: "/*!50003 SELECT 1*/",
			want: "",
		},
		{
			name: "plain /* comment before CREATE PROCEDURE",
			stmt: "/* This is a regular comment */ CREATE PROCEDURE p1() BEGIN END",
			want: "DROP PROCEDURE IF EXISTS `p1`",
		},
		{
			name: "CREATE PROCEDURE wrapped entirely in one version comment",
			stmt: "/*!50003 CREATE PROCEDURE `p1`()\nBEGIN\n  SELECT 1;\nEND */",
			want: "DROP PROCEDURE IF EXISTS `p1`",
		},
		{
			name: "CREATE EVENT wrapped in version comment",
			stmt: "/*!50003 CREATE EVENT `cleanup` ON SCHEDULE EVERY 1 DAY DO DELETE FROM logs */",
			want: "DROP EVENT IF EXISTS `cleanup`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dropGuardStmt([]byte(tt.stmt))
			if tt.want == "" {
				if got != nil {
					t.Errorf("dropGuardStmt() = %q, want nil", string(got))
				}
				return
			}
			if got == nil {
				t.Errorf("dropGuardStmt() = nil, want %q", tt.want)
				return
			}
			if string(got) != tt.want {
				t.Errorf("dropGuardStmt() = %q, want %q", string(got), tt.want)
			}
		})
	}
}
