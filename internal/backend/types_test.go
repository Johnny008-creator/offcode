package backend

import (
	"errors"
	"testing"
)

func TestIsMalformedToolCallResponse(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("backend error: XML syntax error on line 5: element <function> closed by </parameter>"), true},
		{errors.New("backend error: XML syntax error on line 8: unexpected EOF"), true},
		{errors.New("backend returned status 500: internal server error"), false},
		{errors.New("request to http://localhost:11434: connection refused"), false},
	}
	for _, c := range cases {
		if got := IsMalformedToolCallResponse(c.err); got != c.want {
			t.Errorf("IsMalformedToolCallResponse(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
