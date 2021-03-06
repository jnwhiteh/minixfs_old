package minixfs

import (
	. "minixfs/common"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for Finode
//////////////////////////////////////////////////////////////////////////////

type m_finode_req interface {
	is_m_finode_req()
}

type m_finode_res interface {
	is_m_finode_res()
}

// Request types
type m_finode_req_read struct {
	buf []byte
	pos int
}

type m_finode_req_write struct {
	buf []byte
	pos int
}

type m_finode_req_close struct{}
type m_finode_req_lock struct{}
type m_finode_req_unlock struct{}

// Response types
type m_finode_res_io struct {
	n   int
	err error
}

type m_finode_res_asyncio struct {
	callback <-chan m_finode_res_io
}

type m_finode_res_err struct {
	err error
}

type m_finode_res_lock struct {
	finode Finode
}

type m_finode_res_unlock struct {
	ok bool
}

// For type-checking
func (m m_finode_req_read) is_m_finode_req()   {}
func (m m_finode_req_write) is_m_finode_req()  {}
func (m m_finode_req_close) is_m_finode_req()  {}
func (m m_finode_req_lock) is_m_finode_req()   {}
func (m m_finode_req_unlock) is_m_finode_req() {}

func (m m_finode_res_io) is_m_finode_res()      {}
func (m m_finode_res_asyncio) is_m_finode_res() {}
func (m m_finode_res_err) is_m_finode_res()     {}
func (m m_finode_res_lock) is_m_finode_res()    {}
func (m m_finode_res_unlock) is_m_finode_res()  {}

// Check interface implementation
var _ m_finode_req = m_finode_req_read{}
var _ m_finode_req = m_finode_req_write{}
var _ m_finode_req = m_finode_req_close{}
var _ m_finode_req = m_finode_req_lock{}
var _ m_finode_req = m_finode_req_unlock{}

var _ m_finode_res = m_finode_res_io{}
var _ m_finode_res = m_finode_res_asyncio{}
var _ m_finode_res = m_finode_res_err{}
var _ m_finode_res = m_finode_res_lock{}
var _ m_finode_res = m_finode_res_unlock{}
