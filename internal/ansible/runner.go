package ansible

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vu-installer/internal/state"

	log "github.com/sirupsen/logrus"
)

type Runner struct {
	InventoryPath string
	PlaybookPath  string
	UpdateTasks   func(nodeName string, tasks []state.Task)
}

func (r *Runner) GenerateInventory(nodeName, ip, user, keyPath string) error {
	log.WithFields(log.Fields{"node": nodeName, "ip": ip}).Info("Generating Ansible inventory")
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(r.InventoryPath), 0755); err != nil {
		log.WithError(err).Error("Failed to create inventory directory")
		return err
	}

	content := fmt.Sprintf("[%s]\n%s ansible_user=%s ansible_ssh_private_key_file=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n",
		nodeName, ip, user, keyPath)

	err := os.WriteFile(r.InventoryPath, []byte(content), 0644)
	if err != nil {
		log.WithError(err).Error("Failed to write inventory file")
	} else {
		log.Info("Inventory file generated successfully")
	}
	return err
}

func (r *Runner) Run(nodeName, ip string) error {
	log.WithFields(log.Fields{"node": nodeName, "ip": ip}).Info("Starting Ansible playbook execution")
	// Pass Go variables to Ansible as Extra Vars
	extraVars := fmt.Sprintf("private_ip=%s node_hostname=%s", ip, nodeName)

	// Open log file for Ansible output
	logFile, err := os.Create(fmt.Sprintf("ansible_%s.log", nodeName))
	if err != nil {
		log.WithError(err).Error("Failed to create Ansible log file")
		return err
	}
	defer logFile.Close()

	cmd := exec.Command("ansible-playbook",

		"-i", r.InventoryPath,
		r.PlaybookPath,
		"--extra-vars", extraVars,
	)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.WithError(err).Error("Failed to get stdout pipe")
		return err
	}
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	log.Info("Executing Ansible playbook")

	// Start the command
	if err := cmd.Start(); err != nil {
		log.WithError(err).Error("Failed to start Ansible playbook")
		return err
	}

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	tasks := []state.Task{}
	taskRegex := regexp.MustCompile(`^TASK \[(.*)\]`)
	resultRegex := regexp.MustCompile(`^(ok|failed|changed|skipped|unreachable):`)

	for scanner.Scan() {
		line := scanner.Text()
		// Write to stdout and log file
		fmt.Println(line)
		logFile.WriteString(line + "\n")

		// Parse for tasks
		if matches := taskRegex.FindStringSubmatch(line); matches != nil {
			taskName := matches[1]
			tasks = append(tasks, state.Task{Name: taskName, Status: "running"})
			if r.UpdateTasks != nil {
				r.UpdateTasks(nodeName, tasks)
			}
		} else if matches := resultRegex.FindStringSubmatch(line); matches != nil {
			result := matches[1]
			if len(tasks) > 0 {
				lastTask := &tasks[len(tasks)-1]
				switch result {
				case "ok", "changed":
					lastTask.Status = "success"
				case "failed":
					lastTask.Status = "failed"
				case "skipped":
					lastTask.Status = "skipped"
				case "unreachable":
					lastTask.Status = "unreachable"
				}
				if r.UpdateTasks != nil {
					r.UpdateTasks(nodeName, tasks)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.WithError(err).Error("Error reading stdout")
	}

	// Wait for command to finish
	err = cmd.Wait()
	if err != nil {
		log.WithError(err).Error("Ansible playbook execution failed")
	} else {
		log.Info("Ansible playbook executed successfully")
	}
	return err
}

func (r *Runner) MonitorInstallLog(nodeName string) error {
	log.WithField("node", nodeName).Info("Monitoring install_log.txt for completion")
	completionMessage := "Installation completed"
	timeout := 60 // minutes
	for i := 0; i < timeout; i++ {
		cmd := exec.Command("ansible", "-i", r.InventoryPath, nodeName, "-m", "shell", "-a", fmt.Sprintf("grep -q '%s' /home/vunet/vuSmartMaps_offline_NG-3.0/install_log.txt && echo found || echo not", completionMessage))
		output, err := cmd.Output()
		if err != nil {
			log.WithError(err).Error("Failed to check install log")
			return err
		}
		if strings.Contains(string(output), "found") {
			log.Info("Installation completed")
			return nil
		}
		time.Sleep(60 * time.Second)
	}
	return fmt.Errorf("timeout waiting for installation completion")
}

func (r *Runner) Cleanup() {
	log.Info("Cleaning up inventory file")
	err := os.Remove(r.InventoryPath)
	if err != nil {
		log.WithError(err).Warn("Failed to remove inventory file")
	} else {
		log.Info("Inventory file cleaned up")
	}
}