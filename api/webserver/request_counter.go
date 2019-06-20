package webserver

import "strconv"

type requestCounter struct {
	lastId uint64
}

func (c *requestCounter) GetNextId() string {
	strId := strconv.FormatUint(c.lastId, 10)
	c.lastId = c.lastId + 1

	return "REQ-" + strId
}
