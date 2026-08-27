package service

import (
	"task281-citereview/internal/decision"
	"task281-citereview/internal/element"
	"task281-citereview/internal/graph"
	"task281-citereview/internal/material"
	"task281-citereview/internal/scope"
	"task281-citereview/internal/store"
)

// Service 编排层：聚合材料/要素/范围/裁决/研究图五大业务模块与一个 store，
// 向上层（HTTP API、smoke-test）提供统一入口。
type Service struct {
	Store    *store.Store
	Material *material.Service
	Element  *element.Service
	Scope    *scope.Service
	Decision *decision.Service
	Graph    *graph.Service
}

// New 构造编排服务，初始化全部业务模块。
func New(st *store.Store) *Service {
	return &Service{
		Store:    st,
		Material: material.New(st),
		Element:  element.New(st),
		Scope:    scope.New(st),
		Decision: decision.New(st),
		Graph:    graph.New(st),
	}
}
