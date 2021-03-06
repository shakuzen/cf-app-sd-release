package smoke_test

import (
	"time"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	hostName       string
	deletedAppName string
	queryAppName   string
)

var _ = Describe("Delete App Smoke", func() {
	BeforeEach(func() {
		prefix = config.Prefix

		deletedAppName = prefix + "proxyToDelete"
		queryAppName = prefix + "proxyToQuery"

		orgName = prefix + "org" // cf-pusher expects this name
		Expect(cf.Cf("create-org", orgName).Wait(Timeout_Cf)).To(gexec.Exit(0))
		Expect(cf.Cf("target", "-o", orgName).Wait(Timeout_Cf)).To(gexec.Exit(0))

		spaceName := prefix + "space" // cf-pusher expects this name
		Expect(cf.Cf("create-space", spaceName, "-o", orgName).Wait(Timeout_Cf)).To(gexec.Exit(0))
		Expect(cf.Cf("target", "-o", orgName, "-s", spaceName).Wait(Timeout_Cf)).To(gexec.Exit(0))

		By("pushing the app")
		pushProxy(deletedAppName)
		pushProxy(queryAppName)

		By("getting the app guid")
		var proxyGuid string
		session := cf.Cf("app", deletedAppName, "--guid").Wait(2 * time.Second)
		proxyGuid = string(session.Out.Contents())

		By("making sure the app is resolved to the correct ip")
		proxyIPs := []string{}
		hostName = "http://" + queryAppName + "." + config.AppsDomain + "/dig/" + strings.TrimSpace(proxyGuid) + ".apps.internal."
		Eventually(func() []string {
			resp, err := http.Get(hostName)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			ipsJson, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			err = json.Unmarshal(bytes.TrimSpace(ipsJson), &proxyIPs)
			Expect(err).NotTo(HaveOccurred())

			return proxyIPs
		}, 5*time.Second).Should(HaveLen(1))

		var proxyContainerIp string
		session = cf.Cf("ssh", deletedAppName, "-c", "echo $CF_INSTANCE_INTERNAL_IP").Wait(10 * time.Second)
		proxyContainerIp = string(session.Out.Contents())

		Expect(proxyIPs).To(ConsistOf(strings.TrimSpace(proxyContainerIp)))
	})

	AfterEach(func() {
		Expect(cf.Cf("delete-org", orgName, "-f").Wait(Timeout_Cf)).To(gexec.Exit(0))
	})

	Describe("when performing a dns lookup for a domain configured to point to the bosh adapter", func() {
		Measure("does not resolve its infrastructure name within 5 seconds after delete", func(b Benchmarker) {
			By("deleting the app")
			b.Time("delete", func() {
				deleteProxy(deletedAppName)
			})

			By("asserting the dig response is a 500 status code within 5 seconds of app delete finishing")
			b.Time("digAnswer", func() {
				Eventually(func() int {
					resp, err := http.Get(hostName)

					Expect(err).NotTo(HaveOccurred())
					return resp.StatusCode
				}, 5*time.Second).Should(Equal(http.StatusInternalServerError))
			})
		}, 1)
	})

})

func deleteProxy(appName string) {
	Expect(cf.Cf(
		"delete", appName, "-f",
	).Wait(Timeout_Cf)).To(gexec.Exit(0))
}
