// user_enum.go
package schema

type Gender int32

const (
	GenderUnknown Gender = iota
	GenderMale
	GenderFemale
)

func (g Gender) Values() []string {
	return []string{"male", "female", "unknown"}
}

func (g Gender) String() string {
	switch g {
	case GenderMale:
		return "男"
	case GenderFemale:
		return "女"
	default:
		return "未知"
	}
}
