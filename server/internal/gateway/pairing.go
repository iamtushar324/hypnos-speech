package gateway

import (
	"encoding/base64"
	"net/url"

	qrcode "github.com/skip2/go-qrcode"
)

// dictionPairing is revealed only with a newly minted device key. The deep link
// matches Diction's public pairing wire format; neither it nor its QR is logged.
func (s *Server) dictionPairing(token string) (map[string]string, error) {
	if s.cfg.DictionPublicURL == "" {
		return nil, nil
	}
	query := url.Values{"url": {s.cfg.DictionPublicURL}, "key": {token}}
	uri := "diction://pair?" + query.Encode()
	png, err := qrcode.Encode(uri, qrcode.Medium, 512)
	if err != nil {
		return nil, err
	}
	return map[string]string{"endpoint": s.cfg.DictionPublicURL, "pairing_uri": uri, "qr_data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}, nil
}
