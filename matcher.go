package limiter

import (
	"fmt"
)

const (
	skind uint8 = iota
	pkind
	akind
)

type fn interface{}

type node struct {
	kind     uint8
	label    byte
	prefix   string
	parent   *node
	children []*node
	ppath    string
	pnames   []string
	h        handler
	fn       fn
	metadata map[string][]string
}

// method : fn
type handler map[string]fn

type route struct {
	path   string
	metadata map[string][]string
}

// matcher router.
type matcher struct {
	tree   *node
	routes []*route
}

func newRouter() *matcher {
	r := &matcher{
		tree:   &node{
			h: make(handler),
			metadata: make(map[string][]string),
		},
		routes: make([]*route, 0),
	}

	return r
}

func offtake(path string) (raw string, vals []string) {
	vals = []string{}
	for cur, size := 0, len(path); cur < size; cur++ {
		raw = raw + string(path[cur])
		if path[cur] == ':' {
			j := cur + 1
			for ; cur < size && path[cur] != '/'; cur++ {
			}
			vals = append(vals, path[j:cur])
			raw = raw + "/"
		}
	}
	return
}

func (r *matcher) add(path string, metadata map[string][]string, h fn) {
	if path == "" || path[0] != '/' {
		panic("route add")
	}

	var (
		ppath  = path
		pnames = []string{}
	)

	r.routes = append(r.routes, &route{
		// method: method,
		path:   path,
		metadata: metadata,
	})

	for i, l := 0, len(path); i < l; i++ {
		if path[i] == ':' {
			j := i + 1

			r.insert(path[:i], metadata, nil, skind, "", nil)
			for ; i < l && path[i] != '/'; i++ {
			}

			pnames = append(pnames, path[j:i])
			path = path[:j] + path[i:]
			i, l = j, len(path)

			if i == l {
				r.insert(path[:i], metadata, h, pkind, ppath, pnames)
			} else {
				r.insert(path[:i], metadata, nil, pkind, ppath, pnames)
			}

		} else if path[i] == '*' {
			r.insert(path[:i], metadata, nil, skind, "", nil)
			pnames = append(pnames, "_*")
			r.insert(path[:i+1], metadata, h, akind, ppath, pnames)
			return
		}
	}

	r.insert(path, metadata, h, skind, ppath, pnames)
}

func (r *matcher) insert(path string, metadata map[string][]string, h fn, t uint8, ppath string, pnames []string) {
	cn := r.tree // Current node as root
	if cn == nil {
		panic("route insert")

	}
	search := path

	for {
		sl := len(search)
		pl := len(cn.prefix)
		l := 0

		// LCP
		max := pl
		if sl < max {
			max = sl
		}
		for ; l < max && search[l] == cn.prefix[l]; l++ {
		}

		if l == 0 {
			// At root node
			cn.label = search[0]
			cn.prefix = search
			if h != nil {
				cn.kind = t
				// cn.h[method] = h
				cn.fn = h
				cn.metadata = metadata
				cn.ppath = ppath
				cn.pnames = pnames
			}
		} else if l < pl {
			// Split node
			n := newNode(cn.kind, cn.prefix[l:], cn, cn.children, cn.h, cn.ppath, cn.pnames)

			// Reset parent node
			cn.kind = skind
			cn.label = cn.prefix[0]
			cn.prefix = cn.prefix[:l]
			cn.children = nil
			cn.h = make(handler)
			cn.ppath = ""
			cn.pnames = nil

			cn.children = append(cn.children, n)

			if l == sl {
				// At parent node
				cn.kind = t
				cn.fn = h
				cn.metadata = metadata
				cn.ppath = ppath
				cn.pnames = pnames

			} else {
				// Create child node
				n = newNode(t, search[l:], cn, nil, make(handler), ppath, pnames)
				// n.h[method] = h
				n.fn = h
				n.metadata = metadata
				cn.children = append(cn.children, n)

			}

		} else if l < sl {
			search = search[l:]
			c := cn.findByLabel(search[0])
			if c != nil {
				// Go deeper
				cn = c
				continue

			}
			// Create child node
			n := newNode(t, search, cn, nil, make(handler), ppath, pnames)
			// n.h[method] = h
			n.metadata = metadata
			n.fn = h
			cn.children = append(cn.children, n)
		} else {
			// Node already exists
			// PANIC(PATH)
			if h != nil {
				// cn.h[method] = h
				cn.fn = h
				cn.metadata = metadata
				cn.ppath = ppath
				cn.pnames = pnames
			}
		}
		return
	}
}

func (r *matcher) find(path string, metadata map[string]string) (ret *node) {
	cn := r.tree // Current node as root
	ret = &node{}

	var (
		search = path
		c      *node  // Child node
		n      int    // Param counter
		nk     uint8  // Next kind
		nn     *node  // Next node
		ns     string // Next search
	)

	// Search order static > param > any
	for {
		if search == "" {
			goto End
		}

		pl := 0 // Prefix length
		l := 0  // LCP length

		if cn.label != ':' {
			sl := len(search)
			pl = len(cn.prefix)

			// LCP
			max := pl
			if sl < max {
				max = sl
			}
			for ; l < max && search[l] == cn.prefix[l]; l++ {
			}
		}

		if l == pl {
			// Continue search
			search = search[l:]
		} else {
			cn = nn
			search = ns
			if nk == pkind {
				goto Param
			} else if nk == akind {
				goto Any
			}
			return nil
		}

		if search == "" {
			goto End
		}

		// Static node
		if c = cn.find(search[0], skind); c != nil {
			// Save next
			if cn.label == '/' {
				nk = pkind
				nn = cn
				ns = search
			}
			cn = c
			continue
		}

		// Param node
	Param:
		if c = cn.findByKind(pkind); c != nil {
			// Save next
			if cn.label == '/' {
				nk = akind
				nn = cn
				ns = search
			}

			cn = c
			i, l := 0, len(search)
			for ; i < l && search[i] != '/'; i++ {

			}
			n++
			search = search[i:]
			continue
		}
		// Any node
	Any:
		if cn = cn.findByKind(akind); cn == nil {
			if nn != nil {
				cn = nn
				nn = nil // Next
				search = ns
				if nk == pkind {
					goto Param
				} else if nk == akind {
					goto Any
				}
			}
			// Not found
			return nil
		}
		goto End
	}

End:
	fmt.Printf("cn = %+v\n", cn)
	fmt.Printf("metadata = %+v\n", metadata)
	var allMatched = true
	for key, fields := range cn.metadata {
		var matched = false
		if input, ok := metadata[key]; ok {
			fmt.Printf("fields = %+v\n", fields)
			for _, field := range fields {
				if input == field {
					matched = true
					break
				}
			}
		} else {
			matched = true
		}

		allMatched = allMatched && matched
	}
	fmt.Printf("allMatched = %+v\n", allMatched)

	if allMatched {
		ret.fn = cn.fn
	} else {
		ret.fn = nil
	}

	ret.pnames = cn.pnames
	ret.ppath = cn.ppath

	return
}

func newNode(t uint8, pre string, p *node, c []*node, h handler, ppath string, pnames []string) *node {
	return &node{
		kind:     t,
		label:    pre[0],
		prefix:   pre,
		parent:   p,
		children: c,
		ppath:    ppath,
		pnames:   pnames,
		h:        h,
	}
}

func (n *node) find(l byte, t uint8) *node {
	for _, c := range n.children {
		if c.label == l && c.kind == t {
			return c
		}
	}

	return nil
}

func (n *node) findByKind(t uint8) *node {
	for _, c := range n.children {
		if c.kind == t {
			return c
		}
	}

	return nil
}

func (n *node) findByLabel(l byte) *node {
	for _, c := range n.children {
		if c.label == l {
			return c
		}
	}

	return nil
}
