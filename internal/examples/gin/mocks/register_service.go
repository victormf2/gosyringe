package mocks

import (
	"io"

	"github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/setup"
)

func RegisterTestServices(c *gosyringe.Container) {
	setup.RegisterServices(c)

	gosyringe.RegisterScoped[*logrus.Entry](c, func() *logrus.Entry {

		logger := logrus.New()
		logger.Out = io.Discard
		return logrus.NewEntry(logger)
	})
}
