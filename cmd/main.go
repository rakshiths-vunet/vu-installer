package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vu-installer/internal/ansible"
	"vu-installer/internal/state"

	log "github.com/sirupsen/logrus"
)

type InstallRequest struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	User    string `json:"user"`
	Key     string `json:"key"`
	Version string `json:"version"`
	IP1     string `json:"ip1"`
	IP2     string `json:"ip2"`
	IP3     string `json:"ip3"`
	VMName1 string `json:"vmname1"`
	VMName2 string `json:"vmname2"`
	VMName3 string `json:"vmname3"`
}

type InstallResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	// Initialize logging
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.InfoLevel)

	// Initialize database
	dbPath := "installer.db"
	if err := state.InitDB(dbPath); err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}
	log.Info("Database initialized")
	log.Info("Made by Sid & Team - vU-Installer Core")

	// Ansible runner
	runner := &ansible.Runner{
		InventoryPath: "inventory.ini",
		PlaybookPath:  "playbooks/site.yml",
		UpdateTasks: func(nodeName string, tasks []state.Task) {
			s, err := state.Load(nodeName)
			if err != nil {
				log.WithError(err).Error("Failed to load state for task update")
				return
			}
			tasksJSON, err := json.Marshal(tasks)
			if err != nil {
				log.WithError(err).Error("Failed to marshal tasks")
				return
			}
			s.Tasks = sql.NullString{String: string(tasksJSON), Valid: true}
			if len(tasks) > 0 {
				s.Step = sql.NullString{String: tasks[len(tasks)-1].Name, Valid: true}
			}
			if err := state.Save(s); err != nil {
				log.WithError(err).Error("Failed to save tasks")
			}
		},
	}

	// Status handler
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nodeName := r.URL.Query().Get("name")
		if nodeName == "" {
			http.Error(w, "Missing name parameter", http.StatusBadRequest)
			return
		}

		s, err := state.Load(nodeName)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Node not found", http.StatusNotFound)
			} else {
				log.WithError(err).Error("Failed to load state")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		startTimeStr := ""
		if s.StartTime.Valid {
			startTimeStr = s.StartTime.Time.Format(time.RFC3339)
		}

		tasks := []state.Task{}
		if s.Tasks.Valid {
			if err := json.Unmarshal([]byte(s.Tasks.String), &tasks); err != nil {
				log.WithError(err).Error("Failed to unmarshal tasks")
			}
		}

		response := map[string]interface{}{
			"node_name":  s.NodeName,
			"ip":         s.IP.String,
			"status":     s.Status.String,
			"step":       s.Step.String,
			"start_time": startTimeStr,
			"error_msg":  s.ErrorMsg.String,
			"locked":     s.Locked,
			"tasks":      tasks,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Health handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]string{
			"status":    "healthy",
			"developer": "Sid & Team",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Nodes handler
	http.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		states, err := state.GetAll()
		if err != nil {
			log.WithError(err).Error("Failed to get all states")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var nodes []map[string]interface{}
		for _, s := range states {
			startTimeStr := ""
			if s.StartTime.Valid {
				startTimeStr = s.StartTime.Time.Format(time.RFC3339)
			}

			node := map[string]interface{}{
				"node_name":  s.NodeName,
				"ip":         s.IP.String,
				"version":    s.Version.String,
				"status":     s.Status.String,
				"has_vsmaps": s.Status.Valid && s.Status.String == "SUCCESS",
				"start_time": startTimeStr,
				"step":       s.Step.String,
			}
			nodes = append(nodes, node)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes)
	})

	// HTTP handler
	http.HandleFunc("/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.WithError(err).Error("Failed to decode request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Set default SSH key if not provided
		if req.Key == "" {
			home, _ := os.UserHomeDir()
			req.Key = filepath.Join(home, ".ssh", "id_ed25519")
			if _, err := os.Stat(req.Key); err != nil {
				req.Key = filepath.Join(home, ".ssh", "id_rsa")
			}
		}

		// Set default user if not provided
		if req.User == "" {
			req.User = "vunet"
		}

		log.WithFields(log.Fields{"node": req.Name, "ip": req.IP}).Info("Received install request")

		// Load current state
		s, err := state.Load(req.Name)
		if err != nil && err != sql.ErrNoRows {
			log.WithError(err).Error("Failed to load state")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Check if already installed
		if s.Status.Valid && s.Status.String == "SUCCESS" {
			log.WithFields(log.Fields{"node": req.Name}).Info("Node already installed")
			resp := InstallResponse{Status: "success", Message: "Already installed"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Try to lock
		if err := state.LockNode(req.Name); err != nil {
			if err == sql.ErrNoRows {
				log.WithFields(log.Fields{"node": req.Name}).Warn("Node is locked or busy")
				http.Error(w, "Node is busy", http.StatusConflict)
			} else {
				log.WithError(err).Error("Failed to lock node")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Update state to RUNNING
		s = state.InstallState{
			NodeName:  req.Name,
			IP:        sql.NullString{String: req.IP, Valid: true},
			User:      sql.NullString{String: req.User, Valid: true},
			Key:       sql.NullString{String: req.Key, Valid: true},
			Version:   sql.NullString{String: req.Version, Valid: true},
			Status:    sql.NullString{String: "RUNNING", Valid: true},
			Step:      sql.NullString{String: "Starting", Valid: true},
			StartTime: sql.NullTime{Time: time.Now(), Valid: true},
			Tasks:     sql.NullString{String: "[]", Valid: true},
			Locked:    true,
			IP1:       sql.NullString{String: req.IP1, Valid: true},
			IP2:       sql.NullString{String: req.IP2, Valid: true},
			IP3:       sql.NullString{String: req.IP3, Valid: true},
			VMName1:   sql.NullString{String: req.VMName1, Valid: true},
			VMName2:   sql.NullString{String: req.VMName2, Valid: true},
			VMName3:   sql.NullString{String: req.VMName3, Valid: true},
		}
		if err := state.Save(s); err != nil {
			log.WithError(err).Error("Failed to save state")
			state.UnlockNode(req.Name)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Run installation in goroutine to not block
		go func() {
			defer state.UnlockNode(req.Name)

			// Generate inventory
			if err := runner.GenerateInventory(req.Name, req.IP, req.User, req.Key, req.Version, req.IP1, req.IP2, req.IP3, req.VMName1, req.VMName2, req.VMName3); err != nil {
				log.WithError(err).Error("Failed to generate inventory")
				s.Status = sql.NullString{String: "FAILED", Valid: true}
				s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
				state.Save(s)
				return
			}

			// Run playbook
			if err := runner.Run(req.Name, req.IP, req.Version, "", req.IP1, req.IP2, req.IP3, req.VMName1, req.VMName2, req.VMName3); err != nil {
				log.WithError(err).Error("Playbook failed")
				s.Status = sql.NullString{String: "FAILED", Valid: true}
				s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
			} else {
				// Monitor install log for completion
				if err := runner.MonitorInstallLog(req.Name, req.Version); err != nil {
					log.WithError(err).Error("Installation monitoring failed")
					s.Status = sql.NullString{String: "FAILED", Valid: true}
					s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
				} else {
					log.Info("Installation successful")
					s.Status = sql.NullString{String: "SUCCESS", Valid: true}
					s.Step = sql.NullString{String: "Completed", Valid: true}
				}
			}

			s.Locked = false
			state.Save(s)
			runner.Cleanup(req.Name)
		}()

		resp := InstallResponse{Status: "accepted", Message: "Installation started"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Retry handler
	http.HandleFunc("/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.WithError(err).Error("Failed to decode retry request")
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		log.WithFields(log.Fields{"node": req.Name}).Info("Received retry request")

		// Load current state
		s, err := state.Load(req.Name)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Node not found", http.StatusNotFound)
			} else {
				log.WithError(err).Error("Failed to load state")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check if failed
		if !s.Status.Valid || s.Status.String != "FAILED" {
			http.Error(w, "Node is not in failed state", http.StatusConflict)
			return
		}

		// Try to lock
		if err := state.LockNode(req.Name); err != nil {
			if err == sql.ErrNoRows {
				log.WithFields(log.Fields{"node": req.Name}).Warn("Node is locked or busy")
				http.Error(w, "Node is busy", http.StatusConflict)
			} else {
				log.WithError(err).Error("Failed to lock node")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Update state to RUNNING
		s.Status = sql.NullString{String: "RUNNING", Valid: true}
		s.Step = sql.NullString{String: "Retrying", Valid: true}
		s.ErrorMsg = sql.NullString{}
		s.StartTime = sql.NullTime{Time: time.Now(), Valid: true}
		if err := state.Save(s); err != nil {
			log.WithError(err).Error("Failed to save state")
			state.UnlockNode(req.Name)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Run retry in goroutine
		go func() {
			defer state.UnlockNode(req.Name)

			// Generate inventory
			if err := runner.GenerateInventory(req.Name, s.IP.String, s.User.String, s.Key.String, s.Version.String, s.IP1.String, s.IP2.String, s.IP3.String, s.VMName1.String, s.VMName2.String, s.VMName3.String); err != nil {
				log.WithError(err).Error("Failed to generate inventory")
				s.Status = sql.NullString{String: "FAILED", Valid: true}
				s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
				state.Save(s)
				return
			}

			// Find failed task to resume from
			startAt := ""
			var tasks []state.Task
			if s.Tasks.Valid {
				if err := json.Unmarshal([]byte(s.Tasks.String), &tasks); err == nil {
					for _, t := range tasks {
						if t.Status == "failed" || t.Status == "unreachable" || t.Status == "running" {
							startAt = t.Name
							break
						}
					}
				}
			}

			// Run playbook
			if err := runner.Run(req.Name, s.IP.String, s.Version.String, startAt, s.IP1.String, s.IP2.String, s.IP3.String, s.VMName1.String, s.VMName2.String, s.VMName3.String); err != nil {
				log.WithError(err).Error("Playbook failed")
				s.Status = sql.NullString{String: "FAILED", Valid: true}
				s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
			} else {
				// Monitor install log for completion
				if err := runner.MonitorInstallLog(req.Name, s.Version.String); err != nil {
					log.WithError(err).Error("Installation monitoring failed")
					s.Status = sql.NullString{String: "FAILED", Valid: true}
					s.ErrorMsg = sql.NullString{String: err.Error(), Valid: true}
				} else {
					log.Info("Installation successful")
					s.Status = sql.NullString{String: "SUCCESS", Valid: true}
					s.Step = sql.NullString{String: "Completed", Valid: true}
				}
			}

			s.Locked = false
			state.Save(s)
			runner.Cleanup(req.Name)
		}()

		resp := InstallResponse{Status: "accepted", Message: "Retry started"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9091"
	}

	log.WithFields(log.Fields{"port": port}).Info("Starting server")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.WithError(err).Fatal("Server failed")
	}
}
