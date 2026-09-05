package config_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/config"
)

var _ = Describe("Config", func() {
	Describe("Load", func() {
		Context("with default values", func() {
			It("should load config with defaults", func() {
				cfg, err := config.Load()

				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Server.GatewayPort).ToNot(BeEmpty())
			})
		})

		Context("with custom environment variables", func() {
			BeforeEach(func() {
				os.Setenv("GATEWAY_PORT", "9000")
				os.Setenv("DB_HOST", "custom-host")
				os.Setenv("DB_USER", "custom-user")
			})

			AfterEach(func() {
				os.Unsetenv("GATEWAY_PORT")
				os.Unsetenv("DB_HOST")
				os.Unsetenv("DB_USER")
			})

			It("should load config with custom values", func() {
				cfg, err := config.Load()

				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Server.GatewayPort).ToNot(BeEmpty())
			})
		})
	})

	Describe("LoadWithoutEnvFile", func() {
		BeforeEach(func() {
			for _, k := range os.Environ() {
				os.Unsetenv(k)
			}
		})

		It("should load config with default port", func() {
			cfg, err := config.Load()

			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Server.GatewayPort).To(Equal("8080"))
		})
	})

	Describe("getEnv", func() {
		Context("when environment variable is set", func() {
			BeforeEach(func() {
				os.Setenv("TEST_KEY", "env-value")
			})

			AfterEach(func() {
				os.Unsetenv("TEST_KEY")
			})

			It("should return the environment value", func() {
				Expect(config.GetEnv("TEST_KEY", "default")).To(Equal("env-value"))
			})
		})

		Context("when environment variable is not set", func() {
			It("should return the default value", func() {
				Expect(config.GetEnv("TEST_KEY", "default")).To(Equal("default"))
			})
		})
	})
})
