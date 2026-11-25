package proposal

type doc = map[string]any

func obj(v any) doc    { m, _ := v.(map[string]any); return m }
func str(v any) string { s, _ := v.(string); return s }
func num(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
func array(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []doc:
		r := []any{}
		for _, d := range a {
			r = append(r, d)
		}
		return r
	case []string:
		r := []any{}
		for _, s := range a {
			r = append(r, s)
		}
		return r
	}
	return []any{}
}
func items(v any) []doc {
	r := []doc{}
	for _, v := range array(v) {
		r = append(r, obj(v))
	}
	return r
}
func stringsOf(v any) []string {
	r := []string{}
	for _, s := range array(v) {
		r = append(r, str(s))
	}
	return r
}
