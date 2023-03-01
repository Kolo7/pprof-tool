// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Kolo7/pprof-tool/internal/graph"
	"github.com/Kolo7/pprof-tool/internal/measurement"
	"github.com/Kolo7/pprof-tool/internal/report"
)

type Tree struct {
	Self     Node    `json:"node"`
	Parent   []*Node `json:"parent"`
	Children []*Node `json:"c"`
}

type Node struct {
	Name      string `json:"n"`
	FullName  string `json:"f"`
	Cum       int64  `json:"v"`
	CumFormat string `json:"l"`
	Percent   string `json:"p"`
	Tree      *Tree  `json:"-"`
}

func (ui *webInterface) Flamegraph(w http.ResponseWriter, req *http.Request) {

}
func (ui *webInterface) Flamegraph2(w http.ResponseWriter, req *http.Request) ([]string, []*Tree, error) {
	// Force the call tree so that the graph is a tree.
	// Also do not trim the tree so that the flame graph contains all functions.
	rpt, errList := ui.MakeReport(w, req, []string{"svg"}, func(cfg *config) {
		cfg.CallTree = true
		cfg.Trim = false
	})
	if rpt == nil {
		return nil, nil, fmt.Errorf("pprof flamegraph error: %v", errList) // error already reported
	}

	// Generate dot graph.
	g, config := report.GetDOT(rpt)
	var nodes []*Node
	nroots := 0
	rootValue := int64(0)
	nodeArr := []string{}
	nodeMap := map[*graph.Node]*Node{}
	// Make all nodes and the map, collect the roots.
	for _, n := range g.Nodes {
		v := n.CumValue()
		fullName := n.Info.PrintableName()
		node := &Node{
			Name:      graph.ShortenFunctionName(fullName),
			FullName:  fullName,
			Cum:       v,
			CumFormat: config.FormatValue(v),
			Percent:   strings.TrimSpace(measurement.Percentage(v, config.Total)),
		}
		nodes = append(nodes, node)
		if len(n.In) == 0 {
			nodes[nroots], nodes[len(nodes)-1] = nodes[len(nodes)-1], nodes[nroots]
			nroots++
			rootValue += v
		}
		nodeMap[n] = node
		// Get all node names into an array.
		nodeArr = append(nodeArr, n.Info.Name)
	}
	rootTree := &Tree{
		Self: Node{
			Name:      "root",
			FullName:  "root",
			Cum:       rootValue,
			CumFormat: config.FormatValue(rootValue),
			Percent:   strings.TrimSpace(measurement.Percentage(rootValue, config.Total)),
		},
		Children: nodes[0:nroots],
		Parent:   []*Node{},
	}
	rootTree.Self.Tree = rootTree
	// Populate the child links.
	trees := make([]*Tree, 0)
	for _, n := range g.Nodes {
		node := nodeMap[n]
		tree := Tree{
			Self:     *node,
			Parent:   []*Node{},
			Children: []*Node{},
		}
		node.Tree = &tree
		for child := range n.Out {
			tree.Children = append(tree.Children, nodeMap[child])
		}
		for parent := range n.In {
			tree.Parent = append(tree.Parent, nodeMap[parent])
		}
		trees = append(trees, &tree)
	}

	for _, n := range nodes[0:nroots] {
		if n.Tree != nil {
			n.Tree.Parent = append(n.Tree.Parent, &rootTree.Self)
		}
	}
	trees = append(trees, rootTree)

	return nodeArr, trees, nil
}
