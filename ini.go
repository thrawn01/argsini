package argsini

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
	"github.com/thrawn01/args"
)

var KeySeparator string = "/"

type INIBackend struct {
	changeChan chan *args.ChangeEvent
	fileName   string
	section    string
	data       []byte
	file       os.File
	ini        *ini.File
	done       chan struct{}
	wg         sync.WaitGroup
}

// Provide a file and a section to get values from. If no section is provided, key are taken from
// read from values with no section, and sections are treated as argument groups
func NewFromFile(file os.File, section string) args.Backend {
	return &INIBackend{
		file:     file,
		fileName: file.Name(),
		section:  section,
	}, nil
}

func NewFromBuffer(data []byte, fileName, section string) args.Backend {
	return &INIBackend{
		data:     data,
		fileName: fileName,
		section:  section,
	}, nil
}

func (self *INIBackend) loadINI() (err error) {
	if len(self.data) == 0 {
		self.data, err = ioutil.ReadAll(self.file)
		if err != nil {
			return errors.Wrapf(err, "while reading ini '%s'", self.fileName())
		}
	}
	if self.ini == nil {
		self.ini, err = ini.Load(self.data)
	}
	return err
}

// Get retrieves a value specified by the key
func (self *INIBackend) Get(ctx context.Context, key args.Key) (args.Pair, error) {

	if err := self.loadINI(); err != nil {
		return args.Pair{}, err
	}

	if self.section != "" {
		group := self.ini.Section(self.section)
		if group == nil {
			return args.Pair{}, errors.Errorf("non-existant section '%s'", self.section)
		}
		return args.Pair{
			Key:   key,
			Value: []byte(group.Key(key.Join(KeySeparator)).Value()),
		}, nil
	}
	group := self.ini.Section(key.Group)
	if group == nil {
		return args.Pair{}, errors.Errorf("non-existant section '%s'", self.section)
	}
	return args.Pair{
		Key:   key,
		Value: []byte(group.Key(key.Name).Value()),
	}, nil
}

// List retrieves all keys and values under a provided group
func (self *INIBackend) List(ctx context.Context, key args.Key) ([]args.Pair, error) {
	var results []args.Pair

	if err := self.loadINI(); err != nil {
		return args.Pair{}, err
	}

	for _, section := range self.ini.Sections() {
		group := self.ini.Section(section.Name())
		for _, key := range group.KeyStrings() {
			results = append(results, args.Pair{
				Key: args.Key{
					Group: section.Name(),
					Name:  key,
				},
				Value: []byte(group.Key(key).Value()),
			})
		}
	}
	return results, nil
}

// Set the provided key to value.
func (self *INIBackend) Set(ctx context.Context, key args.Key, value []byte) error {
	return errors.New("Not Implemented")
}

// Watch monitors store for changes to key.
func (self *INIBackend) Watch(ctx context.Context, key args.Key) <-chan *args.ChangeEvent {
	var fileEvent chan fileEvent
	var err error

	self.changeChan = make(chan *args.ChangeEvent)
	self.done = make(chan struct{})

	self.wg.Add(1)
	go func() {

		for {
			// Keep trying to watch the file until user tells us to stop
			fileEvent, err = watchFile(ctx, self.fileName, time.Second)
			if err != nil {
				// Send an error
				self.changeChan <- &args.ChangeEvent{
					Err: err,
				}

				// Wait a second and try again
				tick := time.Tick(time.Second)
				select {
				case <-ctx.Done():
					return
				case <-tick:
					continue
				}
			}
			break
		}

		defer self.wg.Done()
		for {
			select {
			case _, ok := <-fileEvent:
				if !ok {
					return
				}
				for _, change := range self.discoverChanges(self.fileName) {
					self.changeChan <- &change
				}
			}
		}
	}()
	return self.changeChan
}

// Return the root key used to store all other keys in the backend
func (self *INIBackend) GetRootKey() string {
	return self.section
}

// Closes the connection to the backend and cancels all watches
func (self *INIBackend) Close() {
	// TODO: Cancel any watches if they exist
}

func (self *INIBackend) discoverChanges(event fileEvent) []args.ChangeEvent {
	var results []args.ChangeEvent

	// Load the file and Determine what changed
	iniFile, err := ini.Load(self.data)
	if err != nil {
		return []args.ChangeEvent{{
			Err: err,
		}}
	}

	// For each item in the existing ini file
	for _, lh := range pairList(self.ini) {

		// Compare with the new ini file
		rh := getPair(lh, iniFile)
		if rh == nil {
			// Deleted event
			results = append(results, args.ChangeEvent{
				Key:     lh.Key,
				Value:   lh.Value,
				Deleted: true,
			})
			continue
		}
		if lh.Value != rh.Value {
			// Value updated event
			results = append(results, args.ChangeEvent{
				Key:   lh.Key,
				Value: lh.Value,
			})
		}
	}

	// For each item in the new ini file
	for _, lh := range pairList(iniFile) {
		// Compare with the old ini file
		rh := getPair(lh, self.ini)
		if rh == nil {
			// Add event
			results = append(results, args.ChangeEvent{
				Key:   lh.Key,
				Value: lh.Value,
			})
		}
	}

	self.ini = iniFile
	return results
}

func pairList(iniFile *ini.File) []args.Pair {
	var results []args.Pair
	for _, section := range iniFile.Sections() {
		group := iniFile.Section(section.Name())
		for key, value := range group.KeysHash() {
			results = append(results, args.Pair{
				Key: args.Key{
					Name:  key,
					Group: section.Name(),
				},
				Value: value,
			})
		}
	}
	return []args.Pair{}
}

func getPair(pair args.Pair, iniFile *ini.File) *args.Pair {
	section, err := iniFile.GetSection(pair.Key.Group)
	if err != nil {
		return nil
	}
	return &args.Pair{
		Key:   pair.Key,
		Value: section.Key(pair.Key.Name).String(),
	}
	return nil
}
