package record

func (r *Record) SetMetaValue(key string, value string) {
	if r.Meta == nil {
		r.Meta = make(map[string]string)
	}
	r.Meta[key] = value
}

func (r *Record) GetMetaValue(key string) (string, bool) {
	if r.Meta == nil {
		return "", false
	}
	v, ok := r.Meta[key]
	return v, ok
}
