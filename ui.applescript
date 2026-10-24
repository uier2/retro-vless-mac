-- native dialogs only; the helper owns VPN state and authorization.
on run argv
	if (count of argv) < 1 then
		display dialog "не указан путь к служебной программе" buttons {"закрыть"} default button "закрыть" with title "retro vless"
		return
	end if
	set helperPath to item 1 of argv
	repeat
		try
			set statusText to my runHelper(helperPath, {"status"})
		on error
			set statusText to "не удалось получить состояние"
		end try
		set actionChoice to choose from list {"подключиться", "отключиться", "добавить ссылку", "выйти"} with title "retro vless" with prompt statusText default items {"подключиться"} OK button name "выбрать" cancel button name "выйти" multiple selections allowed false empty selection allowed false
		set actionName to "выйти"
		if actionChoice is not false then set actionName to item 1 of actionChoice
		try
			if actionName is "подключиться" then
				my connectServer(helperPath)
			else if actionName is "отключиться" then
				set resultText to my runHelper(helperPath, {"disconnect"})
				my showMessage(resultText)
			else if actionName is "добавить ссылку" then
				my importLink(helperPath)
			else
				set answer to display dialog "перед выходом соединение будет отключено. продолжить?" with title "retro vless" buttons {"назад", "отключить и выйти"} default button "назад" cancel button "назад"
				my runHelper(helperPath, {"disconnect"})
				return
			end if
		on error errorText number errorNumber
			if errorNumber is not -128 then my showMessage("операция не выполнена. проверьте ссылку, доступ к сети и разрешение администратора; затем повторите попытку.")
		end try
	end repeat
end run

on runHelper(helperPath, argumentsList)
	set commandText to quoted form of helperPath
	repeat with argumentValue in argumentsList
		set commandText to commandText & " " & quoted form of (argumentValue as text)
	end repeat
	with timeout of 120 seconds
		return do shell script commandText
	end timeout
end runHelper

on connectServer(helperPath)
	set serverText to my runHelper(helperPath, {"list"})
	if serverText is "" then
		my showMessage("серверов пока нет. сначала добавьте ссылку.")
		return
	end if
	set serverRows to paragraphs of serverText
	set serverChoice to choose from list serverRows with title "retro vless" with prompt "выберите сервер; подключение может занять около 15 секунд" OK button name "подключиться" cancel button name "назад" multiple selections allowed false empty selection allowed false
	if serverChoice is false then return
	set selectedRow to item 1 of serverChoice
	set tabPosition to offset of tab in selectedRow
	if tabPosition < 2 then error "invalid server row"
	set serverIndex to text 1 thru (tabPosition - 1) of selectedRow
	-- Validate without changing the CLI-supplied identifier.
	repeat with digitValue in characters of serverIndex
		if "0123456789" does not contain (digitValue as text) then error "invalid server index"
	end repeat
	set resultText to my runHelper(helperPath, {"connect", serverIndex})
	my showMessage(resultText)
end connectServer

on importLink(helperPath)
	set linkDialog to display dialog "вставьте ссылку vless:// или https://. ссылка скрыта при вводе." default answer "" with hidden answer with title "retro vless" buttons {"назад", "добавить"} default button "добавить" cancel button "назад"
	set linkValue to text returned of linkDialog
	considering case
		set validPrefix to (linkValue starts with "vless://") or (linkValue starts with "https://")
	end considering
	if not validPrefix then
		set linkValue to ""
		my showMessage("нужна ссылка, начинающаяся с vless:// или https://")
		return
	end if
	try
		set resultText to my runHelper(helperPath, {"import", linkValue})
	on error
		set linkValue to ""
		-- Never display helper stderr: it might contain the private link.
		my showMessage("не удалось добавить ссылку. проверьте адрес и доступ к сети.")
		return
	end try
	set linkValue to ""
	-- Avoid echoing an imported secret even if helper confirmation includes it.
	my showMessage("ссылка добавлена")
end importLink

on showMessage(messageText)
	display dialog messageText with title "retro vless" buttons {"ок"} default button "ок"
end showMessage
