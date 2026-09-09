package imap

import (
	"fmt"

	"github.com/emersion/go-sasl"
)

// xoauth2 is the SASL mechanism name Google and Microsoft both accept for
// OAuth over IMAP. Microsoft accepts nothing else: outlook.office365.com
// advertises AUTH=XOAUTH2 and not OAUTHBEARER, which go-sasl ships.
const xoauth2 = "XOAUTH2"

// xoauth2Client is the client half of XOAUTH2: one initial response carrying
// the user and the bearer token, and an empty reply to any challenge, which
// is how the server delivers its JSON error before the final tagged NO.
type xoauth2Client struct {
	username, token string
	failed          bool
}

func newXOAuth2Client(username, token string) sasl.Client {
	return &xoauth2Client{username: username, token: token}
}

func (c *xoauth2Client) Start() (string, []byte, error) {
	ir := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.token)
	return xoauth2, []byte(ir), nil
}

func (c *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	// A challenge here is the server's error document; answering it with an
	// empty line lets the exchange end on the server's NO rather than hang.
	if c.failed {
		return nil, fmt.Errorf("xoauth2: authentication refused: %s", string(challenge))
	}
	c.failed = true
	return []byte{}, nil
}
