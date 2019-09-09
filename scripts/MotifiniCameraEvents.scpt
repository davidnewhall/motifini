-- Use this script with SecuritySpy to send events into motifini when cameras detect motion.

-- Change Gate to a real camera name to test this in Script Editor
property TestCam : "Gate"
property Motifini : "http://127.0.0.1:8765"

on run arg
	if (count of arg) is not 2 then set arg to {0, TestCam}
	set Camera to item 2 of arg -- item 1 is the cam number.
	do shell script ("curl -s -X POST -A SecuritySpy " & Motifini & "'/api/v1.0/event/notify/" & Camera & "'")
end run
