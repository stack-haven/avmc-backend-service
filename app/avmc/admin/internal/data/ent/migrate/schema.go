// Code generated by ent, DO NOT EDIT.

package migrate

import (
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"
)

var (
	// DeptsColumns holds the columns for the "depts" table.
	DeptsColumns = []*schema.Column{
		{Name: "id", Type: field.TypeInt, Increment: true},
	}
	// DeptsTable holds the schema information for the "depts" table.
	DeptsTable = &schema.Table{
		Name:       "depts",
		Columns:    DeptsColumns,
		PrimaryKey: []*schema.Column{DeptsColumns[0]},
	}
	// MenusColumns holds the columns for the "menus" table.
	MenusColumns = []*schema.Column{
		{Name: "id", Type: field.TypeInt, Increment: true},
	}
	// MenusTable holds the schema information for the "menus" table.
	MenusTable = &schema.Table{
		Name:       "menus",
		Columns:    MenusColumns,
		PrimaryKey: []*schema.Column{MenusColumns[0]},
	}
	// PostsColumns holds the columns for the "posts" table.
	PostsColumns = []*schema.Column{
		{Name: "id", Type: field.TypeInt, Increment: true},
	}
	// PostsTable holds the schema information for the "posts" table.
	PostsTable = &schema.Table{
		Name:       "posts",
		Columns:    PostsColumns,
		PrimaryKey: []*schema.Column{PostsColumns[0]},
	}
	// RolesColumns holds the columns for the "roles" table.
	RolesColumns = []*schema.Column{
		{Name: "id", Type: field.TypeInt, Increment: true},
	}
	// RolesTable holds the schema information for the "roles" table.
	RolesTable = &schema.Table{
		Name:       "roles",
		Columns:    RolesColumns,
		PrimaryKey: []*schema.Column{RolesColumns[0]},
	}
	// UsersColumns holds the columns for the "users" table.
	UsersColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUint32, Increment: true, SchemaType: map[string]string{"mysql": "bigint", "postgres": "serial"}},
		{Name: "created_at", Type: field.TypeTime},
		{Name: "updated_at", Type: field.TypeTime},
		{Name: "deleted_at", Type: field.TypeTime, Nullable: true},
		{Name: "domain_id", Type: field.TypeUint32, SchemaType: map[string]string{"mysql": "bigint", "postgres": "bigint"}},
		{Name: "name", Type: field.TypeString, Unique: true, Size: 32},
		{Name: "password", Type: field.TypeString, Size: 100},
		{Name: "realname", Type: field.TypeString, Nullable: true, Size: 50},
		{Name: "nickname", Type: field.TypeString, Nullable: true, Size: 50},
		{Name: "email", Type: field.TypeString, Unique: true, Nullable: true, Size: 100},
		{Name: "phone", Type: field.TypeString, Unique: true, Nullable: true, Size: 20},
		{Name: "avatar", Type: field.TypeString, Nullable: true, Size: 255},
		{Name: "birthday", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"mysql": "date"}},
		{Name: "gender", Type: field.TypeInt32, Default: 0, SchemaType: map[string]string{"mysql": "tinyint"}},
		{Name: "age", Type: field.TypeInt, Nullable: true},
		{Name: "status", Type: field.TypeInt32, Default: 1, SchemaType: map[string]string{"mysql": "tinyint"}},
		{Name: "last_login_at", Type: field.TypeTime, Nullable: true},
		{Name: "last_login_ip", Type: field.TypeString, Nullable: true, Size: 50},
		{Name: "login_count", Type: field.TypeInt, Default: 0},
		{Name: "settings", Type: field.TypeJSON, Nullable: true},
		{Name: "metadata", Type: field.TypeJSON, Nullable: true},
		{Name: "description", Type: field.TypeString, Nullable: true, Size: 255},
	}
	// UsersTable holds the schema information for the "users" table.
	UsersTable = &schema.Table{
		Name:       "users",
		Columns:    UsersColumns,
		PrimaryKey: []*schema.Column{UsersColumns[0]},
		Indexes: []*schema.Index{
			{
				Name:    "user_id",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[0]},
			},
			{
				Name:    "user_domain_id",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[4]},
			},
			{
				Name:    "user_name",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[5]},
			},
			{
				Name:    "user_phone",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[10]},
			},
			{
				Name:    "user_status",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[15]},
			},
			{
				Name:    "user_email",
				Unique:  false,
				Columns: []*schema.Column{UsersColumns[9]},
			},
		},
	}
	// Tables holds all the tables in the schema.
	Tables = []*schema.Table{
		DeptsTable,
		MenusTable,
		PostsTable,
		RolesTable,
		UsersTable,
	}
)

func init() {
}
