// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020 Nick Pillitteri
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package roger

import (
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func SetupLogger(l level.Option) log.Logger {
	logger := log.NewLogfmtLogger(os.Stderr)
	logger = level.NewFilter(logger, l)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	return logger
}
