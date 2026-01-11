package internal

import "errors"

var ErrSlugExists = errors.New("slug already exists")
var ErrLinkNotFound = errors.New("link not found")

