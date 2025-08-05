package entgo

func WithPagination[Q interface {
	Limit(int) Q
	Offset(int) Q
}](q Q, page int, pageSize int) Q {
	if page <= 0 {
		page = 1
	}
	q = q.Limit(pageSize).Offset(pageSize * (page - 1))
	return q
}
