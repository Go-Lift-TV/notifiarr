package notifiarr

func (c *Config) logSnapshotStartup() {
	var ex string

	for k, v := range map[string]bool{
		"raid":    c.Snap.Raid,
		"disks":   c.Snap.DiskUsage,
		"drives":  c.Snap.DriveData,
		"uptime":  c.Snap.Uptime,
		"cpumem":  c.Snap.CPUMem,
		"cputemp": c.Snap.CPUTemp,
		"zfs":     c.Snap.ZFSPools != nil,
		"sudo":    c.Snap.UseSudo && c.Snap.DriveData,
	} {
		if !v {
			continue
		}

		if ex != "" {
			ex += ", "
		}

		ex += k
	}

	c.Printf("==> System Snapshot Collection Started, interval: %v, timeout: %v, enabled: %s",
		c.Snap.Interval, c.Snap.Timeout, ex)
}

func (t *Triggers) SendSnapshot(event EventType) {
	if t.stop == nil {
		return
	}

	t.snap.C <- event
}

func (c *Config) sendSnapshot(event EventType) {
	snapshot, errs, debug := c.Snap.GetSnapshot()
	for _, err := range errs {
		if err != nil {
			c.Errorf("[%s requested] Snapshot: %v", event, err)
		}
	}

	// These debug messages are mostly just errors that we we expect to have.
	for _, err := range debug {
		if err != nil {
			c.Debugf("Snapshot: %v", err)
		}
	}

	resp, err := c.SendData(SnapRoute.Path(event), &Payload{Snap: snapshot}, true)
	if err != nil {
		c.Errorf("[%s requested] Sending snapshot to Notifiarr: %v", event, err)
	} else {
		c.Printf("[%s requested] System Snapshot sent to Notifiarr, cron interval: %s. "+
			"Website took %s and replied with: %s, %s",
			event, c.Snap.Interval, resp.Message.Elapsed, resp.Status, resp.Message.Response)
	}
}
