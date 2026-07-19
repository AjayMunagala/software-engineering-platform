package ignore

import "errors"

var ErrDiscoveryRequired = errors.New("Ignore Engine requires Discovery Engine output")
