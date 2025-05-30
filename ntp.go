package kvm

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/beevik/ntp"
)

const (
	timeSyncRetryStep     = 5 * time.Second
	timeSyncRetryMaxInt   = 1 * time.Minute
	timeSyncWaitNetChkInt = 100 * time.Millisecond
	timeSyncWaitNetUpInt  = 3 * time.Second
	timeSyncInterval      = 1 * time.Hour
	timeSyncTimeout       = 2 * time.Second
)

var (
	builtTimestamp        string
	timeSyncRetryInterval = 0 * time.Second
	timeSyncSuccess       = false
	defaultNTPServers     = []string{
		"time.cloudflare.com",
		"time.apple.com",
	}
)

func isTimeSyncNeeded() bool {
	if builtTimestamp == "" {
		logger.Warnf("Built timestamp is not set, time sync is needed")
		return true
	}

	ts, err := strconv.Atoi(builtTimestamp)
	if err != nil {
		logger.Warnf("Failed to parse built timestamp: %v", err)
		return true
	}

	// builtTimestamp is UNIX timestamp in seconds
	builtTime := time.Unix(int64(ts), 0)
	now := time.Now()

	logger.Tracef("Built time: %v, now: %v", builtTime, now)

	if now.Sub(builtTime) < 0 {
		logger.Warnf("System time is behind the built time, time sync is needed")
		return true
	}

	return false
}

func TimeSyncLoop() {
	for {
		if !networkState.checked {
			time.Sleep(timeSyncWaitNetChkInt)
			continue
		}

		if !networkState.Up {
			logger.Infof("Waiting for network to come up")
			time.Sleep(timeSyncWaitNetUpInt)
			continue
		}

		// check if time sync is needed, but do nothing for now
		isTimeSyncNeeded()

		logger.Infof("Syncing system time")
		start := time.Now()
		err := SyncSystemTime()
		if err != nil {
			logger.Warnf("Failed to sync system time: %v", err)

			// retry after a delay
			timeSyncRetryInterval += timeSyncRetryStep
			time.Sleep(timeSyncRetryInterval)
			// reset the retry interval if it exceeds the max interval
			if timeSyncRetryInterval > timeSyncRetryMaxInt {
				timeSyncRetryInterval = 0
			}

			continue
		}
		timeSyncSuccess = true
		logger.Infof("Time sync successful, now is: %v, time taken: %v", time.Now(), time.Since(start))
		time.Sleep(timeSyncInterval) // after the first sync is done
	}
}

func SyncSystemTime() (err error) {
	now, err := queryNetworkTime()
	if err != nil {
		return fmt.Errorf("failed to query network time: %w", err)
	}
	err = setSystemTime(*now)
	if err != nil {
		return fmt.Errorf("failed to set system time: %w", err)
	}
	return nil
}

func queryNetworkTime() (*time.Time, error) {
	ntpServers, err := getNTPServersFromDHCPInfo()
	if err != nil {
		logger.Warnf("failed to get NTP servers from DHCP info: %v\n", err)
	}

	if ntpServers == nil {
		ntpServers = defaultNTPServers
		logger.Infof("Using default NTP servers: %v\n", ntpServers)
	} else {
		logger.Infof("Using NTP servers from DHCP: %v\n", ntpServers)
	}

	for _, server := range ntpServers {
		now, err := queryNtpServer(server, timeSyncTimeout)
		if err == nil {
			logger.Infof("NTP server [%s] returned time: %v\n", server, now)
			return now, nil
		}
	}
	httpUrls := []string{
		"http://apple.com",
		"http://cloudflare.com",
	}
	for _, url := range httpUrls {
		now, err := queryHttpTime(url, timeSyncTimeout)
		if err == nil {
			return now, nil
		}
	}
	return nil, errors.New("failed to query network time")
}

func queryNtpServer(server string, timeout time.Duration) (now *time.Time, err error) {
	resp, err := ntp.QueryWithOptions(server, ntp.QueryOptions{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return &resp.Time, nil
}

func queryHttpTime(url string, timeout time.Duration) (*time.Time, error) {
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Head(url)
	if err != nil {
		return nil, err
	}
	dateStr := resp.Header.Get("Date")
	now, err := time.Parse(time.RFC1123, dateStr)
	if err != nil {
		return nil, err
	}
	return &now, nil
}

func setSystemTime(now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	output, err := exec.Command("date", "-s", nowStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run date -s: %w, %s", err, string(output))
	}
	return nil
}
