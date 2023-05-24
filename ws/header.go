package ws

const (
	HeaderFieldID      = "id"
	HeaderFieldVersion = "version"
	HeaderFieldAction  = "action"
)

type (
	Header map[string]any
)

func (h Header) Key() string {
	version, action := h.Version(), h.Action()

	if version == "" {
		return action
	}
	return version + "/" + action
}

func (h Header) ID() string {
	id, _ := h.GetString(HeaderFieldID)
	return id
}

func (h Header) SetID(id string) {
	h.Set(HeaderFieldID, id)
}

func (h Header) Version() string {
	version, _ := h.GetString(HeaderFieldVersion)
	return version
}

func (h Header) SetVersion(version string) {
	h.Set(HeaderFieldVersion, version)
}

func (h Header) Action() string {
	action, _ := h.GetString(HeaderFieldAction)
	return action
}

func (h Header) SetAction(action string) {
	h.Set(HeaderFieldAction, action)
}

func (h Header) GetString(field string) (string, bool) {
	return GetHeaderWithType[string](h, field)
}

func (h Header) Get(field string) (any, bool) {
	v, ok := h[field]
	return v, ok
}

func (h Header) Set(field string, val any) {
	h[field] = val
}

func (h Header) Delete(field string) {
	delete(h, field)
}

func GetHeaderWithType[T any](h Header, field string) (val T, ok bool) {
	v, ok := h[field]
	if !ok {
		return val, false
	}
	v1, ok := v.(T)
	if !ok {
		return val, ok
	}
	return v1, true
}
