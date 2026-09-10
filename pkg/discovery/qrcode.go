package discovery

import (
	"fmt"

	"github.com/skip2/go-qrcode"
)

// render a QR code for the given URL to the terminal
// unicode block, no image file
func PrintQR(url string) error {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("failed to generate QR code: %w", err)
	}
	fmt.Println(qr.ToString(false))
	return nil
}

// return the QR code for the given URL as PNG bytes
func QRCodePNG(url string, size int) ([]byte, error) {
	png, err := qrcode.Encode(url, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("failed to encode QR PNG: %w", err)
	}
	return png, nil
}
