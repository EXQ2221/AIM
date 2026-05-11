[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$base = "http://localhost:8080"
$pass = "Passw0rd!"
$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$userA = @{ aim_id = "smoke_a_$ts"; email = "smoke_a_$ts@example.com"; nickname = "冒烟A$ts"; password = $pass }
$userB = @{ aim_id = "smoke_b_$ts"; email = "smoke_b_$ts@example.com"; nickname = "冒烟B$ts"; password = $pass }

$cookieA = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$cookieB = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$results = New-Object System.Collections.Generic.List[object]

function Add-Result($name, $ok, $status, $detail) {
  $results.Add([pscustomobject]@{ name=$name; ok=$ok; status=$status; detail=$detail }) | Out-Null
}

function Call-Api {
  param([string]$Name,[string]$Method,[string]$Path,$Body=$null,$Session=$null)
  try {
    $params = @{ Uri=("$base$Path"); Method=$Method; UseBasicParsing=$true; ErrorAction='Stop' }
    if ($Session) { $params.WebSession = $Session }
    if ($Body -ne $null) {
      $params.ContentType = 'application/json'
      $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress)
    }
    $resp = Invoke-WebRequest @params
    $data = $null
    if ($resp.Content) { try { $data = $resp.Content | ConvertFrom-Json } catch {} }
    Add-Result $Name $true $resp.StatusCode 'OK'
    return @{ ok=$true; data=$data; resp=$resp }
  } catch {
    $status = -1
    $msg = $_.Exception.Message
    if ($_.Exception.Response) {
      try { $status = [int]$_.Exception.Response.StatusCode } catch {}
      try { $reader = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); $body = $reader.ReadToEnd(); if ($body) { $msg = "$msg | $body" } } catch {}
    }
    Add-Result $Name $false $status $msg
    return @{ ok=$false; data=$null; resp=$null }
  }
}

Call-Api 'healthz' 'GET' '/healthz' | Out-Null
Call-Api 'auth.register.A' 'POST' '/api/v1/auth/register' $userA | Out-Null
$loginA = Call-Api 'auth.login.A' 'POST' '/api/v1/auth/login' @{ email=$userA.email; password=$pass; device_name='smoke-a' } $cookieA
Call-Api 'auth.register.B' 'POST' '/api/v1/auth/register' $userB | Out-Null
Call-Api 'auth.login.B' 'POST' '/api/v1/auth/login' @{ email=$userB.email; password=$pass; device_name='smoke-b' } $cookieB | Out-Null

$meA = Call-Api 'users.me.A' 'GET' '/api/v1/users/me' $null $cookieA
$meB = Call-Api 'users.me.B' 'GET' '/api/v1/users/me' $null $cookieB
$userBId = $null
if ($meB.ok) { $userBId = [int64]$meB.data.data.user_id }

Call-Api 'auth.sessions.A' 'GET' '/api/v1/auth/sessions' $null $cookieA | Out-Null
$refreshTokenA = $null
if ($loginA.ok -and $loginA.resp -and $loginA.resp.Headers -and $loginA.resp.Headers['Set-Cookie']) {
  $setCookie = [string]$loginA.resp.Headers['Set-Cookie']
  $m = [regex]::Match($setCookie, 'refresh_token=([^;,\s]+)')
  if ($m.Success) { $refreshTokenA = $m.Groups[1].Value }
}
if ([string]::IsNullOrWhiteSpace($refreshTokenA)) {
  Call-Api 'auth.refresh.A' 'POST' '/api/v1/auth/refresh' @{} $cookieA | Out-Null
} else {
  Call-Api 'auth.refresh.A' 'POST' '/api/v1/auth/refresh' @{ refresh_token = $refreshTokenA } $cookieA | Out-Null
}
Call-Api 'friends.groups.create.A' 'POST' '/api/v1/friends/groups' @{ name='默认分组' } $cookieA | Out-Null
Call-Api 'friends.groups.list.A' 'GET' '/api/v1/friends/groups' $null $cookieA | Out-Null
Call-Api 'friends.add.AtoB' 'POST' '/api/v1/friends' @{ target_aim_id=$userB.aim_id; remark='测试好友' } $cookieA | Out-Null
$reqsB = Call-Api 'friends.requests.list.B' 'GET' '/api/v1/friends/requests' $null $cookieB
if ($reqsB.ok -and $reqsB.data.data -and $reqsB.data.data.Count -gt 0) {
  $rid = $reqsB.data.data[0].id
  Call-Api 'friends.requests.respond.B' 'POST' "/api/v1/friends/requests/$rid/respond" @{ action='ACCEPTED' } $cookieB | Out-Null
}
$friendsA = Call-Api 'friends.list.A' 'GET' '/api/v1/friends' $null $cookieA
if ($friendsA.ok -and $friendsA.data.data -and $friendsA.data.data.Count -gt 0) {
  $fid = $friendsA.data.data[0].user_id
  Call-Api 'friends.update.A' 'PATCH' "/api/v1/friends/$fid" @{ remark='已更新备注' } $cookieA | Out-Null
  Call-Api 'friends.delete.A' 'DELETE' "/api/v1/friends/$fid" $null $cookieA | Out-Null
}

$group = Call-Api 'conversations.group.create.A' 'POST' '/api/v1/conversations/group' @{ name='冒烟群'; announcement='测试公告'; joinPolicy='INVITE_ONLY' } $cookieA
$cid = $null
if ($group.ok) { $cid = [string]$group.data.data.conversationId }
Call-Api 'conversations.list.A' 'GET' '/api/v1/conversations' $null $cookieA | Out-Null
if ($userBId) { Call-Api 'conversations.single.find.AB' 'GET' ("/api/v1/conversations/single?targetUserId=" + $userBId) $null $cookieA | Out-Null }
if ($cid) {
  Call-Api 'conversations.group.info' 'GET' "/api/v1/conversations/$cid/group" $null $cookieA | Out-Null
  Call-Api 'conversations.members.list.A' 'GET' "/api/v1/conversations/$cid/members" $null $cookieA | Out-Null
  if ($userBId) {
    Call-Api 'conversations.members.invite.B' 'POST' "/api/v1/conversations/$cid/members/invite" @{ targetUserId=$userBId } $cookieA | Out-Null
  }
  Call-Api 'conversations.muteall.enable' 'POST' "/api/v1/conversations/$cid/mute-all" $null $cookieA | Out-Null
  Call-Api 'conversations.muteall.disable' 'DELETE' "/api/v1/conversations/$cid/mute-all" $null $cookieA | Out-Null
  Call-Api 'conversations.announcement.update' 'PUT' "/api/v1/conversations/$cid/announcement" @{ announcement='公告更新' } $cookieA | Out-Null
  $msgList = Call-Api 'conversations.messages.list' 'GET' "/api/v1/conversations/$cid/messages" $null $cookieA
  $lastReadMessageID = $null
  if ($msgList.ok -and $msgList.data.data -and $msgList.data.data.Count -gt 0) {
    $lastReadMessageID = [int64]$msgList.data.data[0].id
  }
  if ($lastReadMessageID -gt 0) {
    Call-Api 'conversations.read.mark' 'POST' "/api/v1/conversations/$cid/read" @{ lastReadMessageId = $lastReadMessageID } $cookieA | Out-Null
  }
  Call-Api 'conversations.bots.list' 'GET' "/api/v1/conversations/$cid/bots" $null $cookieA | Out-Null
  $bots = Call-Api 'bots.list' 'GET' '/api/v1/bots' $null $cookieA
  if ($bots.ok -and $bots.data.data -and $bots.data.data.Count -gt 0) {
    $botId = $bots.data.data[0].botId
    Call-Api 'conversations.bots.add' 'POST' "/api/v1/conversations/$cid/bots" @{ botId=$botId } $cookieA | Out-Null
    Call-Api 'conversations.bots.remove' 'DELETE' "/api/v1/conversations/$cid/bots/$botId" $null $cookieA | Out-Null
  }
  Call-Api 'conversations.ai.logs' 'GET' "/api/v1/conversations/$cid/ai-call-logs" $null $cookieA | Out-Null
}

Call-Api 'auth.logout.B' 'POST' '/api/v1/auth/logout' $null $cookieB | Out-Null
Call-Api 'auth.logoutall.A' 'POST' '/api/v1/auth/logout-all' @{ password=$pass } $cookieA | Out-Null

$passed = @($results | Where-Object { $_.ok }).Count
$failed = @($results | Where-Object { -not $_.ok }).Count
Write-Output "=== AIM API Smoke Test Summary ==="
Write-Output ("Total: {0}, Passed: {1}, Failed: {2}" -f $results.Count, $passed, $failed)
Write-Output "--- Failed Cases ---"
$failRows = $results | Where-Object { -not $_.ok }
if ($failRows.Count -eq 0) { Write-Output "(none)" } else { $failRows | ForEach-Object { Write-Output ("{0} | status={1} | {2}" -f $_.name, $_.status, $_.detail) } }
Write-Output "--- All Cases ---"
$results | ForEach-Object { Write-Output ("{0} | {1} | status={2} | {3}" -f $_.name, ($(if($_.ok){'PASS'}else{'FAIL'})), $_.status, $_.detail) }
