package id

import (
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kratos/kratos/v2/log"
)

type ID[T any] interface {
	Generate() int64
	Parse() T
}

// NewSnowflake 生成雪花算法id
func NewSnowflake(logger log.Logger, node ...int64) *snowflake.Node {
	l := log.NewHelper(log.With(logger, "module", "snowflake/data/initialize"))
	os.Hostname()
	sf, err := snowflake.NewNode(1)
	if err != nil {
		l.Fatal("snowflake failed init")
	}
	l.Infof("init snowflake ID：%s", sf.Generate())
	return sf
}
