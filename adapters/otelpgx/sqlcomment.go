package otelpgx

import (
	"net/url"
	"sort"
	"strings"
)

// commentFields holds the keys we currently emit in the sqlcommenter
// prefix. New fields are added here; the encoder sorts alphabetically.
type commentFields struct {
	application string
	route       string
	traceparent string
}

// encode returns the Google sqlcommenter prefix for the given fields,
// or the empty string if all fields are empty. Output format:
//
//	/*key1='value1',key2='value2',...*/
//
// Values are URL-encoded (net/url.QueryEscape, which escapes "'" to
// "%27"); keys are alphabetised so Cloud SQL Insights matches
// deterministically.
func encode(f commentFields) string {
	pairs := make([]string, 0, 3)
	add := func(k, v string) {
		if v == "" {
			return
		}
		pairs = append(pairs, k+"='"+strings.ReplaceAll(url.QueryEscape(v), "+", "%20")+"'")
	}
	add("application", f.application)
	add("route", f.route)
	add("traceparent", f.traceparent)
	if len(pairs) == 0 {
		return ""
	}
	sort.Strings(pairs) // alphabetical because "key=" prefix sorts on key
	var b strings.Builder
	b.WriteString("/*")
	b.WriteString(strings.Join(pairs, ","))
	b.WriteString("*/")
	return b.String()
}
