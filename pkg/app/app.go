// Roger - DNS and network metrics exporter for Prometheus
//
// Copyright 2020 Nick Pillitteri
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// http://www.apache.org/licenses/LICENSE-2.0> or the MIT license
// <LICENSE-MIT or http://opensource.org/licenses/MIT>, at your
// option. This file may not be copied, modified, or distributed
// except according to those terms.

package app

import (
	log "github.com/sirupsen/logrus"
)

var Log = setupLogger()

func setupLogger() *log.Logger {
	logger := log.New()
	logger.SetReportCaller(true)
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	return logger
}
