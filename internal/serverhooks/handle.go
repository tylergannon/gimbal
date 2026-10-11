// Package serverhooks supplies the standard native-app build integration point.
// Project ownership is supplied by web.Instance before the SKGo request boundary.
package serverhooks

import "github.com/tylergannon/gimbal/internal/skgo/params"

var Handle = params.Sequence()
