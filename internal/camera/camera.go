// Package camera reports which cameras exist and whether each is in use.
package camera

// Camera is one video device as macOS reports it.
type Camera struct {
	Name  string
	UID   string
	InUse bool
}

// InUse reports whether the configured camera is in use. When uid is set it
// matches on UID only; otherwise on Name. Any matching camera in use counts,
// and no match counts as not in use, so a missing camera is "not in use".
func InUse(cams []Camera, name, uid string) bool {
	for _, c := range cams {
		matches := c.Name == name
		if uid != "" {
			matches = c.UID == uid
		}
		if matches && c.InUse {
			return true
		}
	}
	return false
}
