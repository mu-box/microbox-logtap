package logtap

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/pagodabox/golang-hatchet"
	"net/http"
	"strconv"
)

// HistoricalDrain matches the drain interface
type HistoricalDrain struct {
	port string
	max  int
	log  hatchet.Logger
	db   *bolt.DB
	deploy []string
}

// NewHistoricalDrain returns a new instance of a HistoricalDrain
func NewHistoricalDrain(port string, file string, max int) *HistoricalDrain {
	db, err := bolt.Open(file, 0644, nil)
	if err != nil {
		db, err = bolt.Open("./bolt.db", 0644, nil)
	}
	return &HistoricalDrain{
		port: port,
		max:  max,
		db:   db,
	}
}

// allow us to clear history of the deploy logs
func (h *HistoricalDrain) ClearDeploy() {
	h.deploy = []string{}
}

// Start starts the http listener.
// The listener on every request returns a json hash of logs of some arbitrary size
// default size is 100
func (h *HistoricalDrain) Start() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/logtap/system", h.handlerSystem)
		mux.HandleFunc("/logtap/deploy", h.handlerDeploy)
		err := http.ListenAndServe(":"+h.port, mux)
		if err != nil {
			h.log.Error("[LOGTAP]"+err.Error())
		}
	}()
}

// Handle deploys that come into this drain
// deploy logs should stay relatively short and should be cleared out easily
func (h *HistoricalDrain) handlerDeploy(w http.ResponseWriter, r *http.Request) {
	for _, msg := range h.deploy {
		fmt.Fprintf(w, "%s", msg)
	}
}

// handlerSystem handles any web request with any path and returns logs
// this makes it so a client that talks to pagodabox's logvac
// can communicate with this system
func (h *HistoricalDrain) handlerSystem(w http.ResponseWriter, r *http.Request) {
	var limit int64
	if i, err := strconv.ParseInt(r.FormValue("limit"), 10, 64); err == nil {
		limit = i
	} else {
		limit = 10000
	}
	h.log.Debug("[LOGTAP][handler] limit: %d", limit)
	h.db.View(func(tx *bolt.Tx) error {
		// Create a new bucket.
		b := tx.Bucket([]byte("log"))
		c := b.Cursor()

		// move the curser along so we can start dropping logs
		// in the right order at the right place
		if int64(b.Stats().KeyN) > limit {
			c.First()
			move_forward := int64(b.Stats().KeyN) - limit
			for i := int64(1); i < move_forward; i++ {
				c.Next()
			}
		} else {
			c.First()
		}

		for k, v := c.Next(); k != nil; k, v = c.Next() {
			fmt.Fprintf(w, "%s - %s", k, v)
		}

		return nil
	})

}

// SetLogger really allows the logtap main struct
// to assign its own logger to the historical drain
func (h *HistoricalDrain) SetLogger(l hatchet.Logger) {
	h.log = l
}

// Write is used to implement the interface and do 
// type switching
func (h *HistoricalDrain) Write(msg Message) {
  switch msg.Type {
  case "deploy":
  	h.WriteDeploy(msg)
  default :
    h.WriteSystem(msg)
  }
}

// Write deploy logs to the deploy array.
// much quicker and better at handling deploy logs
func (h *HistoricalDrain) WriteDeploy(msg Message) {
	h.deploy = append(h.deploy, (msg.Time.String()+" - "+msg.Content))
}

// WriteSyslog drops data into a capped collection of logs
// if we hit the limit the last log item will be removed from the beginning
func (h *HistoricalDrain) WriteSystem(msg Message) {
	h.log.Debug("[LOGTAP][Historical][write] message: (%s)%s", msg.Time.String(), msg.Content)
	h.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			h.log.Error("[LOGTAP][Historical][write]" + err.Error())
			return err
		}
		err = bucket.Put([]byte(msg.Time.String()), []byte(msg.Content))
		if err != nil {
			h.log.Error("[LOGTAP][Historical][write]" + err.Error())
			return err
		}

		if bucket.Stats().KeyN > h.max {
			delete_count := bucket.Stats().KeyN - h.max
			c := bucket.Cursor()
			for i := 0; i < delete_count; i++ {
				c.First()
				c.Delete()
			}
		}

		return nil
	})

}
