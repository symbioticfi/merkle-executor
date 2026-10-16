package keeper

import (
	"encoding/json"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

// iterationInventory belongs to one serialized Tick. Its jobs are private copies
// of durable state, and reservations change only after successful persistence.
// Any uncertain write invalidates the entire view: the next Tick reloads disk.
type iterationInventory struct {
	jobs     map[string]Job
	accounts map[common.Address]map[string]struct{}
	valid    bool
}

// Inventory precedes job creation so removing a configured strategy cannot hide
// an external order after its authorization transaction has been acknowledged.
func (k *Keeper) inventory() ([]string, error) {
	var names []string
	if _, err := k.Store.Load("strategies.v1", &names); err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(names)+len(k.Strategies))
	for _, name := range names {
		known[name] = true
	}
	changed := false
	for _, s := range k.Strategies {
		if !known[s.Config.Name] {
			names = append(names, s.Config.Name)
			known[s.Config.Name] = true
			changed = true
		}
	}
	if changed {
		slices.Sort(names)
		if err := k.Store.Save("strategies.v1", names); err != nil {
			return nil, err
		}
	}
	return names, nil
}

func (k *Keeper) loadInventory() (*iterationInventory, error) {
	names, err := k.inventory()
	if err != nil {
		return nil, err
	}
	configured := make(map[string]Strategy, len(k.Strategies))
	for _, s := range k.Strategies {
		configured[s.Config.Name] = s
	}
	pending := ""
	if k.Sender != nil {
		pending = k.Sender.PendingID()
	}
	matched := pending == ""
	view := &iterationInventory{jobs: make(map[string]Job, len(names)), accounts: make(map[common.Address]map[string]struct{}), valid: true}
	for _, name := range names {
		var job Job
		found, err := k.Store.Load(key(name), &job)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		s, exists := configured[name]
		if !exists {
			if job.Phase != "settled" && job.Phase != "failed" {
				return nil, errors.Errorf("active strategy %s removed from configuration", name)
			}
		} else if err := validateJob(job, s); err != nil {
			return nil, err
		}
		if pending != "" && job.ID == pending {
			if !exists {
				return nil, errors.Errorf("transaction owner strategy %s removed from configuration", name)
			}
			matched = true
		}
		view.replace(name, job)
	}
	if !matched {
		return nil, errors.New("unresolved transaction has no configured workflow; restore its strategy configuration")
	}
	return view, nil
}

func (v *iterationInventory) accountBusy(s Strategy) bool {
	owners := v.accounts[common.HexToAddress(s.Config.Account)]
	_, own := owners[s.Config.Name]
	return len(owners) > 1 || (len(owners) == 1 && !own)
}

func (v *iterationInventory) job(name string) (Job, bool, error) {
	job, found := v.jobs[name]
	if !found {
		return Job{}, false, nil
	}
	copy, err := copyInventoryJob(job)
	if err != nil {
		v.valid = false
	}
	return copy, true, err
}

func (v *iterationInventory) save(store Store, name string, job Job) error {
	// Copy before writing: provider maps/slices must not alias the authoritative
	// view, even when a later progress update mutates the caller's local job.
	copy, err := copyInventoryJob(job)
	if err == nil {
		err = store.Save(key(name), job)
	}
	if err != nil {
		v.valid = false
		return err
	}
	v.replace(name, copy)
	return nil
}

func (v *iterationInventory) delete(store Store, name string) error {
	if err := store.Delete(key(name)); err != nil {
		v.valid = false
		return err
	}
	v.remove(name)
	return nil
}

func (v *iterationInventory) replace(name string, job Job) {
	v.remove(name)
	v.jobs[name] = job
	if job.Phase == "prepared" || job.Phase == "authorized" {
		account := job.Plan.Snapshot.Account
		if v.accounts[account] == nil {
			v.accounts[account] = make(map[string]struct{})
		}
		v.accounts[account][name] = struct{}{}
	}
}

func (v *iterationInventory) remove(name string) {
	if job, found := v.jobs[name]; found {
		account := job.Plan.Snapshot.Account
		delete(v.accounts[account], name)
		if len(v.accounts[account]) == 0 {
			delete(v.accounts, account)
		}
		delete(v.jobs, name)
	}
}

func copyInventoryJob(job Job) (Job, error) {
	// Use the journal representation so future nested plan/provider fields remain
	// isolated automatically, without maintaining a second deep-copy schema.
	data, err := json.Marshal(job)
	if err != nil {
		return Job{}, errors.Errorf("copy durable job: %w", err)
	}
	var copy Job
	if err := json.Unmarshal(data, &copy); err != nil {
		return Job{}, errors.Errorf("copy durable job: %w", err)
	}
	return copy, nil
}
