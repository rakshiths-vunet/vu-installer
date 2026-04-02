package state

import (
	"database/sql"
	// "encoding/json"

	_ "github.com/mattn/go-sqlite3"
)

type Task struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type InstallState struct {
	NodeName  string
	IP        sql.NullString
	User      sql.NullString
	Key       sql.NullString
	Version   sql.NullString
	Status    sql.NullString
	Step      sql.NullString
	ErrorMsg  sql.NullString
	StartTime sql.NullTime
	Tasks     sql.NullString
	Locked    bool
	IP1       sql.NullString
	IP2       sql.NullString
	IP3       sql.NullString
	VMName1   sql.NullString
	VMName2   sql.NullString
	VMName3   sql.NullString
}

var db *sql.DB

func InitDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS install_states (
		node_name TEXT PRIMARY KEY,
		ip TEXT,
		status TEXT,
		step TEXT,
		start_time DATETIME,
		error_msg TEXT,
		tasks TEXT,
		locked INTEGER DEFAULT 0,
		version TEXT,
		user TEXT,
		key TEXT,
		ip1 TEXT,
		ip2 TEXT,
		ip3 TEXT,
		vmname1 TEXT,
		vmname2 TEXT,
		vmname3 TEXT
	)`)
	return err
}

func Save(s InstallState) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO install_states (node_name, ip, status, step, start_time, error_msg, tasks, locked, version, user, key, ip1, ip2, ip3, vmname1, vmname2, vmname3) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.NodeName, s.IP.String, s.Status.String, s.Step.String, s.StartTime.Time, s.ErrorMsg.String, s.Tasks.String, s.Locked, s.Version.String, s.User.String, s.Key.String, s.IP1.String, s.IP2.String, s.IP3.String, s.VMName1.String, s.VMName2.String, s.VMName3.String)
	return err
}

func Load(nodeName string) (InstallState, error) {
	var s InstallState
	var locked int
	err := db.QueryRow(`SELECT node_name, ip, status, step, start_time, error_msg, tasks, locked, version, user, key, ip1, ip2, ip3, vmname1, vmname2, vmname3 FROM install_states WHERE node_name = ?`, nodeName).Scan(
		&s.NodeName, &s.IP, &s.Status, &s.Step, &s.StartTime, &s.ErrorMsg, &s.Tasks, &locked, &s.Version, &s.User, &s.Key, &s.IP1, &s.IP2, &s.IP3, &s.VMName1, &s.VMName2, &s.VMName3)
	s.Locked = locked == 1
	return s, err
}

func GetAll() ([]InstallState, error) {
	rows, err := db.Query(`SELECT node_name, ip, status, step, start_time, error_msg, tasks, locked, version, user, key FROM install_states`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []InstallState
	for rows.Next() {
		var s InstallState
		var locked int
		err := rows.Scan(&s.NodeName, &s.IP, &s.Status, &s.Step, &s.StartTime, &s.ErrorMsg, &s.Tasks, &locked, &s.Version, &s.User, &s.Key, &s.IP1, &s.IP2, &s.IP3, &s.VMName1, &s.VMName2, &s.VMName3)
		if err != nil {
			return nil, err
		}
		s.Locked = locked == 1
		states = append(states, s)
	}
	return states, rows.Err()
}

func LockNode(nodeName string) error {
	// Try to lock an existing unlocked node
	result, err := db.Exec(`UPDATE install_states SET locked = 1 WHERE node_name = ? AND locked = 0`, nodeName)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		return nil // Successfully locked existing node
	}

	// Check if node exists and is locked
	var locked int
	err = db.QueryRow(`SELECT locked FROM install_states WHERE node_name = ?`, nodeName).Scan(&locked)
	if err == nil {
		if locked == 1 {
			return sql.ErrNoRows // Already locked
		}
		// If locked == 0, but update didn't work? Unlikely
		return sql.ErrNoRows
	}
	if err != sql.ErrNoRows {
		return err
	}

	// Node doesn't exist, insert with locked=1
	_, err = db.Exec(`INSERT INTO install_states (node_name, locked) VALUES (?, 1)`, nodeName)
	return err
}

func UnlockNode(nodeName string) error {
	_, err := db.Exec(`UPDATE install_states SET locked = 0 WHERE node_name = ?`, nodeName)
	return err
}
