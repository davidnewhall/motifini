(* ***************************************************************
**  --== Motifini Messages.app AppleScript handler ==--   **
**  Copy this to ~/Library/Application Scripts/com.apple.iChat/ **
**  Activate it in Messages.app Preferences.                    **
**  Requires 10.13.3 or earlier, support removed in 10.13.4     **
*************************************************************** *)

using terms from application "Messages"
	on message received theMessage from theBuddy for theChat
		set theHandle to handle of theBuddy
		do shell script ("curl -s -X POST -A iMessageRelay 'http://127.0.0.1:8765/api/v1.0/recv/imessage/msg/" & theHandle & "' --data-urlencode 'msg=" & theMessage & "'")
	end message received

	-- When first message is received, accept the invitation.
	on received text invitation theMessage from theBuddy for theChat
		accept theChat
	end received text invitation
	on received audio invitation theText from theBuddy for theChat
		decline theChat
	end received audio invitation
	on received video invitation theText from theBuddy for theChat
		decline theChat
	end received video invitation
	on received file transfer invitation theFileTransfer
		decline theFileTransfer
	end received file transfer invitation
	on buddy authorization requested theRequest
		accept theRequest
	end buddy authorization requested
	on message sent theMessage for theChat
	end message sent
	on chat room message received theMessage from theBuddy for theChat
	end chat room message received
	on active chat message received theMessage
	end active chat message received
	on addressed chat room message received theMessage from theBuddy for theChat
	end addressed chat room message received
	on addressed message received theMessage from theBuddy for theChat
	end addressed message received
	on av chat started
	end av chat started
	on av chat ended
	end av chat ended
	on login finished for theService
	end login finished
	on logout finished for theService
	end logout finished
	on buddy became available theBuddy
	end buddy became available
	on buddy became unavailable theBuddy
	end buddy became unavailable
	on completed file transfer
	end completed file transfer
end using terms from
