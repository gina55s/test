package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/identity"
	"github.com/threefoldtech/test/pkg/provision"
	"github.com/urfave/cli"
)

func cmdsLive(c *cli.Context) error {
	var (
		seedPath = c.String("seed")
		start    = c.Int("start")
		end      = c.Int("end")
		expired  = c.Bool("expired")
		deleted  = c.Bool("deleted")
	)

	keypair, err := identity.LoadKeyPair(seedPath)
	if err != nil {
		return errors.Wrapf(err, "could not find seed file at %s", seedPath)
	}

	s := scraper{
		poolSize: 10,
		start:    start,
		end:      end,
		expired:  expired,
		deleted:  deleted,
	}

	cResults := s.Scrap(keypair.Identity())
	for result := range cResults {
		printResult(result)
	}
	return nil
}

const timeLayout = "02-Jan-2006 15:04:05"

func printResult(r res) {
	expire := r.Created.Add(r.Duration)
	fmt.Printf("ID:%6s Type:%10s expired at:%20s", r.ID, r.Type, expire.Format(timeLayout))
	if r.Result == nil {
		fmt.Printf("state: not deployed yet\n")
		return
	}
	fmt.Printf("state: %6s", r.Result.State)
	if r.Result.State == "error" {
		fmt.Printf("\t%s\n", r.Result.Error)
		return
	}

	switch r.Type {
	case provision.VolumeReservation:
		rData := provision.VolumeResult{}
		data := provision.Volume{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			panic(err)
		}
		if err := json.Unmarshal(r.Result.Data, &rData); err != nil {
			panic(err)
		}
		fmt.Printf("\tVolume ID: %s Size: %d Type: %s\n", rData.ID, data.Size, data.Type)
	case provision.ZDBReservation:
		data := provision.ZDBResult{}
		if err := json.Unmarshal(r.Result.Data, &data); err != nil {
			panic(err)
		}
		fmt.Printf("\tAddr %s:%d Namespace %s\n", data.IP, data.Port, data.Namespace)

	case provision.ContainerReservation:
		data := provision.Container{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			panic(err)
		}
		fmt.Printf("\tflist: %s", data.FList)
		for _, ip := range data.Network.IPs {
			fmt.Printf("\tIP: %s", ip)
		}
		fmt.Printf("\n")
	case provision.NetworkReservation:
		data := pkg.Network{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			panic(err)
		}

		fmt.Printf("\tnetwork ID: %s\n", data.Name)

	case provision.KubernetesReservation:
		data := provision.Kubernetes{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			panic(err)
		}

		fmt.Printf("\tip: %v", data.IP)
		if data.MasterIPs == nil || len(data.MasterIPs) == 0 {
			fmt.Print(" master\n")
		} else {
			fmt.Printf("\n")
		}
	}
}

type scraper struct {
	poolSize int
	start    int
	end      int
	expired  bool
	deleted  bool
	wg       sync.WaitGroup
}
type job struct {
	id      int
	user    string
	expired bool
	deleted bool
}
type res struct {
	provision.Reservation
	Result *provision.Result `json:"result"`
}

func (s *scraper) Scrap(user string) chan res {

	var (
		cIn  = make(chan job)
		cOut = make(chan res)
	)

	s.wg.Add(s.poolSize)
	for i := 0; i < s.poolSize; i++ {
		go worker(&s.wg, cIn, cOut)
	}

	go func() {
		defer func() {
			close(cIn)
		}()
		for i := s.start; i < s.end; i++ {
			cIn <- job{
				id:      i,
				user:    user,
				expired: s.expired,
			}
		}
	}()

	go func() {
		s.wg.Wait()
		close(cOut)
	}()

	return cOut
}

func worker(wg *sync.WaitGroup, cIn <-chan job, cOut chan<- res) {
	defer func() {
		wg.Done()
	}()

	for job := range cIn {
		res, err := getResult(job.id)
		if err != nil {
			continue
		}

		if !job.expired && res.Expired() {
			continue
		}
		if !job.deleted && res.ToDelete {
			continue
		}
		if res.User != job.user {
			continue
		}
		cOut <- res
	}
}

func getResult(id int) (res, error) {
	url := fmt.Sprintf("https://explorer.devnet.grid.tf/reservations/%d-1", id)
	resp, err := http.Get(url)
	if err != nil {
		return res{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return res{}, os.ErrNotExist
	}

	if resp.StatusCode != http.StatusOK {
		return res{}, fmt.Errorf("wrong status code %s", resp.Status)
	}

	b := res{}
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return res{}, err
	}

	return b, nil
}
