package shared

import (
	"fmt"
	"strconv"
	"strings"
)

type Synsetpath struct {
	Path []int
}

func ParseSynsetpath(s string) (Synsetpath, error) {
	parts := strings.Split(s, ".")
	path := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return Synsetpath{}, fmt.Errorf("invalid synsetpath: %s", s)
		}
		if num < 0 {
			return Synsetpath{}, fmt.Errorf("negative number not allowed in synsetpath: %s", s)
		}
		path[i] = num
	}
	return Synsetpath{Path: path}, nil
}

func (sp Synsetpath) String() string {
	parts := make([]string, len(sp.Path))
	for i, num := range sp.Path {
		parts[i] = strconv.Itoa(num)
	}
	return strings.Join(parts, ".")
}
