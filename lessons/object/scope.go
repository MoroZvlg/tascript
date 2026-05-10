package object

type Scope struct {
	store map[string]ObjectType
	outer *Scope
}

func NewScope() *Scope {
	return &Scope{
		store: make(map[string]ObjectType),
	}
}

func NewEnclosedScope(outer *Scope) *Scope {
	if outer == nil {
		outer = NewScope()
	}
	return &Scope{
		store: make(map[string]ObjectType),
		outer: outer,
	}
}

func (e *Scope) Get(name string) (ObjectType, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Scope) Set(name string, val ObjectType) ObjectType {
	e.store[name] = val
	return val
}
