!macro customCheckAppRunning
  StrCpy $R1 0
  prime_wait_for_exit:
  ${nsProcess::FindProcess} "${APP_EXECUTABLE_FILENAME}" $R0
  ${If} $R0 == 0
    IntOp $R1 $R1 + 1
    ${If} $R1 < 60
      Sleep 500
      Goto prime_wait_for_exit
    ${EndIf}
    SetErrorLevel 4
    Quit
  ${EndIf}
!macroend
