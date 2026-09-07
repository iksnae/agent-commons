// SPDX-License-Identifier: MPL-2.0

package codexlaunch

// Close releases the cooperating binding lock without removing checkpoints or
// the persistent lock file. Like the checkpoint methods, it is single-owner.
func (j *DirectoryJournal) Close() error {
	j.closed = true
	if j.lease != nil {
		return j.lease.Close()
	}
	return nil
}
