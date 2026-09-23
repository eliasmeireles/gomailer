package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Recipients is a list of email addresses. In JSON it accepts either a comma-separated string
// ("a@x.com, b@x.com") or an array of strings (["a@x.com", "b@x.com"]); entries are trimmed and
// blanks dropped. It is always marshalled as an array.
type Recipients []string

// ParseRecipients splits a comma-separated list into Recipients, dropping blank entries.
func ParseRecipients(list string) Recipients {
	var recipients Recipients
	for address := range strings.SplitSeq(list, ",") {
		if trimmed := strings.TrimSpace(address); trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	return recipients
}

// UnmarshalJSON accepts a comma-separated string, an array of strings or null.
func (r *Recipients) UnmarshalJSON(data []byte) error {
	var list string
	if err := json.Unmarshal(data, &list); err == nil {
		*r = ParseRecipients(list)
		return nil
	}

	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("recipients must be a string or an array of strings: %w", err)
	}

	*r = ParseRecipients(strings.Join(items, ","))
	return nil
}

// String joins the recipients with ", ", as used in the To/Cc headers and logs.
func (r Recipients) String() string {
	return strings.Join(r, ", ")
}
