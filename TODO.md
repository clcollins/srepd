TO DO

* Cache incident info when collected
* Standardize error messages
* Replace panic() where possible 
* Remove most/all functional stuff to commands.go and trigger everything via tea.Msg/tea.Cmd
* Add tests for all pd.go functions
* Add tests for all the commands.go functions


// Re-implement input areas
if m.input.Focused() {

			// Command for focused "input" textarea
			switch {
			case key.Matches(msg, defaultKeyMap.Enter):
				// TODO: SAVE INPUT TO VARIABLE HERE WHEN ENTER IS PRESSED
				m.input.SetValue("")
				m.input.Blur()

			case key.Matches(msg, defaultKeyMap.Back):
				m.input.SetValue("")
				m.input.Blur()
			}

			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

