package model

import (
	"path"
	"strconv"
	"strings"
)

func TransferCopyName(name string, number int) string {
	extension := path.Ext(name)
	if extension == name {
		extension = ""
	}
	return strings.TrimSuffix(name, extension) + strconv.Itoa(number) + extension
}
