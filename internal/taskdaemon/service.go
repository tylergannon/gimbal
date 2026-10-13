package taskdaemon

func (d *Daemon) workdir(requested string) string {
	if d.Workdir != "" {
		return d.Workdir
	}
	return requested
}
