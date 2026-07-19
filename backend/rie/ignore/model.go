package ignore

import (
	"regexp"

	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

// Summary aliases the shared public ignore report.
type Summary = rie.IgnoreSummary

type rule struct {
	pattern   string
	basePath  string
	source    string
	negated   bool
	directory bool
	matcher   *regexp.Regexp
}
