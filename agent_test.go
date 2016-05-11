package gorelic

import (
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

type WaveMetrica struct {
	sawtoothMax     int
	sawtoothCounter int
}

func (metrica *WaveMetrica) GetName() string {
	return "Custom/Wave_Metrica"
}

func (metrica *WaveMetrica) GetUnits() string {
	return "Queries/Second"
}

func (metrica *WaveMetrica) GetValue() (float64, error) {
	metrica.sawtoothCounter++
	if metrica.sawtoothCounter > metrica.sawtoothMax {
		metrica.sawtoothCounter = 0
	}
	return float64(metrica.sawtoothCounter), nil
}

var _ = Describe("Agent", func() {
	Describe("Without license set", func() {
		var agent *Agent

		BeforeEach(func() {
			agent = NewAgent()
		})

		Describe("NewAgent", func() {
			Context("With no parameters", func() {
				It("should create a new agent struct", func() {
					Expect(agent).To(BeAssignableToTypeOf(&Agent{}))
				})
			})
		})

		Describe("Run", func() {
			Context("With no license set", func() {
				It("should fail to run because License isn't set", func() {
					Expect(agent.Run()).To(MatchError(errors.New("please, pass a valid newrelic license key")))
				})
			})
		})
	})

	Describe("With license set", func() {
		var agent *Agent

		BeforeEach(func() {
			agent = NewAgent()
			agent.NewrelicLicense = "YOUR NEWRELIC LICENSE KEY THERE"
		})

		Describe("Run", func() {
			Context("With license set", func() {
				It("should succeed", func() {
					Expect(agent.Run()).To(Succeed())
				})
			})
		})

		Describe("WrapHTTPHandlerFunc", func() {
			handlerfunc := func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "Hi there, I love bacon!")
			}
			req, _ := http.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			Context("When HTTP handler succeeds", func() {
				It("should run without error", func() {
					wrappedFunc := agent.WrapHTTPHandlerFunc(handlerfunc, "/")
					wrappedFunc(w, req)
					Expect(w.Code).To(Equal(http.StatusOK))
				})
			})
		})

		Describe("WrapHTTPHandler", func() {
			handlerfunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "Hi there, I love bacon!")
			})

			req, _ := http.NewRequest("GET", "", nil)
			w := httptest.NewRecorder()

			Context("When HTTP handler succeeds", func() {
				It("should run without error", func() {
					wrappedFunc := agent.WrapHTTPHandler(handlerfunc)
					wrappedFunc.ServeHTTP(w, req)
					Expect(w.Code).To(Equal(http.StatusOK))
				})
			})
		})

		Describe("AddCustomMetric", func() {
			Context("When using the custom WaveMetrica from examples", func() {
				It("Should add the custom metric to the agent", func() {
					wm := &WaveMetrica{
						sawtoothMax:     10,
						sawtoothCounter: 5,
					}
					agent.AddCustomMetric(wm)
					Expect(len(agent.CustomMetrics)).To(Equal(1))
					metricValue, err := agent.CustomMetrics[0].GetValue()
					Expect(metricValue).To(Equal(6.0))
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
