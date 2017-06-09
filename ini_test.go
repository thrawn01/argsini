package argsini_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thrawn01/args"
	"github.com/thrawn01/argsini"
)

type TestLogger struct {
	result string
}

func NewTestLogger() *TestLogger {
	return &TestLogger{""}
}

func (self *TestLogger) Print(stuff ...interface{}) {
	self.result = self.result + fmt.Sprint(stuff...) + "|"
}

func (self *TestLogger) Printf(format string, stuff ...interface{}) {
	self.result = self.result + fmt.Sprintf(format, stuff...) + "|"
}

func (self *TestLogger) Println(stuff ...interface{}) {
	self.result = self.result + fmt.Sprintln(stuff...) + "|"
}

func (self *TestLogger) GetEntry() string {
	return self.result
}

var _ = Describe("argsini", func() {
	var log *TestLogger

	BeforeEach(func() {
		log = NewTestLogger()
	})

	Describe("NewFromBuffer()", func() {
		It("Should provide arg values from INI file", func() {
			parser := args.NewParser()
			parser.AddFlag("--one").IsString()
			parser.Log(log)

			input := []byte("one=this is one value\ntwo=this is two value\n")
			opt, err := parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(BeNil())
			Expect(opt.String("one")).To(Equal("this is one value"))
		})

		It("Should provide arg values from INI file after parsing the command line", func() {
			parser := args.NewParser()
			parser.AddFlag("--one").IsString()
			parser.AddFlag("--two").IsString()
			parser.AddFlag("--three").IsString()
			cmdLine := []string{"--three", "this is three value"}
			opt, err := parser.Parse(cmdLine)
			input := []byte("one=this is one value\ntwo=this is two value\n")
			opt, err = parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(BeNil())
			Expect(opt.String("one")).To(Equal("this is one value"))
			Expect(opt.String("three")).To(Equal("this is three value"))
		})

		It("Should not overide options supplied via the command line", func() {
			parser := args.NewParser()
			parser.AddFlag("--one").IsString()
			parser.AddFlag("--two").IsString()
			parser.AddFlag("--three").IsString()
			cmdLine := []string{"--three", "this is three value", "--one", "this is from the cmd line"}
			opt, err := parser.Parse(cmdLine)
			input := []byte("one=this is one value\ntwo=this is two value\n")
			opt, err = parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(BeNil())
			Expect(opt.String("one")).To(Equal("this is from the cmd line"))
			Expect(opt.String("three")).To(Equal("this is three value"))
		})
		It("Should clear any pre existing slices in the struct before assignment", func() {
			parser := args.NewParser()
			var list []string
			parser.AddFlag("--list").StoreStringSlice(&list).Default("foo,bar,bit")

			opt, err := parser.Parse(nil)
			Expect(err).To(BeNil())
			Expect(opt.StringSlice("list")).To(Equal([]string{"foo", "bar", "bit"}))
			Expect(list).To(Equal([]string{"foo", "bar", "bit"}))

			input := []byte("list=six,five,four\n")
			opt, err = parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(BeNil())
			Expect(opt.StringSlice("list")).To(Equal([]string{"six", "five", "four"}))
			Expect(list).To(Equal([]string{"six", "five", "four"}))
		})
		It("Should raise an error if a Config is required but not provided", func() {
			parser := args.NewParser()
			parser.AddConfig("one").Required()
			input := []byte("two=this is one value\nthree=this is two value\n")
			_, err := parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(Equal("config 'one' is required"))
		})
		It("Should not raise if options and configs share the same name, but are in diff groups", func() {
			parser := args.NewParser()
			parser.AddFlag("--debug").IsTrue()
			parser.AddConfig("debug").InGroup("database").IsBool()

			cmdLine := []string{"--debug"}
			opt, err := parser.Parse(cmdLine)

			iniFile := []byte(`
				[database]
				debug=false
			`)
			opt, err = parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", iniFile, ""))
			Expect(err).To(BeNil())
			Expect(opt.Bool("debug")).To(Equal(true))
			Expect(opt.Group("database").Bool("debug")).To(Equal(false))
		})
		It("Should report IsSet properly", func() {
			parser := args.NewParser()
			parser.AddFlag("--one")
			parser.AddFlag("--two")
			parser.AddConfig("three")
			parser.AddFlag("four")
			parser.AddConfig("five")

			// 'two' is missing from the command line
			cmdLine := []string{"--one", "this is one"}
			opt, err := parser.Parse(cmdLine)
			Expect(opt.String("one")).To(Equal("this is one"))
			Expect(opt.IsSet("one")).To(Equal(true))
			Expect(opt.IsSet("two")).To(Equal(false))
			Expect(opt.IsSet("three")).To(Equal(false))
			Expect(opt.IsSet("four")).To(Equal(false))
			Expect(opt.IsSet("five")).To(Equal(false))

			input := []byte("two=this is two value\nthree=yes")
			opt, err = parser.FromBackend(argsini.NewFromBuffer("/tmp/fake-file.txt", input, ""))
			Expect(err).To(BeNil())
			Expect(opt.IsSet("two")).To(Equal(true))
			Expect(opt.IsSet("one")).To(Equal(true))
			Expect(opt.IsSet("three")).To(Equal(true))
			Expect(opt.IsSet("four")).To(Equal(false))
			Expect(opt.IsSet("five")).To(Equal(false))

			err = opt.Required([]string{"two", "one", "three"})
			Expect(err).To(BeNil())
			err = opt.Required([]string{"two", "one", "four"})
			Expect(err).To(Not(BeNil()))
		})
		It("Should parse an adhoc group from the ini file", func() {
			cmdLine := []string{"--one", "one-thing"}
			parser := args.NewParser()
			parser.Log(log)
			parser.AddFlag("--one").IsString()
			parser.AddConfigGroup("candy-bars")

			opt, err := parser.Parse(cmdLine)
			Expect(err).To(BeNil())
			Expect(log.GetEntry()).To(Equal(""))
			Expect(opt.String("one")).To(Equal("one-thing"))

			iniFile := []byte(`
				one=true

				[candy-bars]
				snickers=300 Cals
				fruit-snacks=100 Cals
				m&ms=400 Cals
			`)
			backend := argsini.NewFromBuffer("/tmp/fake-file.txt", iniFile, "")
			opt, err = parser.FromBackend(backend)
			Expect(err).To(BeNil())
			Expect(opt.Group("candy-bars").ToMap()).To(Equal(map[string]interface{}{
				"snickers":     "300 Cals",
				"fruit-snacks": "100 Cals",
				"m&ms":         "400 Cals",
			}))

		})
	})
	Describe("argsini.NewFromFile()", func() {
		It("Should watch ini file for new values", func() {
			parser := args.NewParser()
			parser.Log(log)
			parser.AddConfigGroup("endpoints")

			backend := argsini.NewFromFile("/tmp/fake-file.txt", ""))
			opts, err := parser.FromBackend(backend)

			Expect(err).To(BeNil())
			Expect(log.GetEntry()).To(Equal(""))
			Expect(opts.Group("endpoints").ToMap()).To(Equal(map[string]interface{}{
				"endpoint1": "http://endpoint1.com:3366",
			}))

			done := make(chan struct{})
			cancelWatch := parser.Watch(backend, func(event *args.ChangeEvent, err error) {
				// Always check for errors
				if err != nil {
					fmt.Printf("Watch Error - %s\n", err.Error())
					close(done)
					return
				}
				parser.Apply(opts.FromChangeEvent(event))
				// Tell the test to continue, Change event was handled
				close(done)
			})
			// TODO: Add a new endpoint to the ini file

			// Wait until the change event is handled
			<-done
			// Stop the watch
			cancelWatch()
			// Get the updated options
			opts = parser.GetOpts()

			Expect(log.GetEntry()).To(Equal(""))
			Expect(opts.Group("endpoints").ToMap()).To(Equal(map[string]interface{}{
				"endpoint1": "http://endpoint1.com:3366",
				"endpoint2": "http://endpoint2.com:3366",
			}))
		})
		// TODO
		It("Should apply any change using opt.FromChangeEvent()", func() {})
	})

})
