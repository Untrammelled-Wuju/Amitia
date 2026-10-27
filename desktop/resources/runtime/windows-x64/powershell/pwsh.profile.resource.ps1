## Copyright (c) Microsoft Corporation. All rights reserved.
## Licensed under the MIT License.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('get', 'set', 'export')]
    [string]$Operation,
    [Parameter(ValueFromPipeline)]
    [string[]]$UserInput
)

Begin {
    enum ProfileType {
        AllUsersCurrentHost
        AllUsersAllHosts
        CurrentUserAllHosts
        CurrentUserCurrentHost
    }

    function New-PwshResource {
        param(
            [Parameter(Mandatory = $true)]
            [ProfileType] $ProfileType,

            [Parameter(ParameterSetName = 'WithContent')]
            [string] $Content,

            [Parameter(ParameterSetName = 'WithContent')]
            [bool] $Exist
        )

        # Create the PSCustomObject with properties
        $resource = [PSCustomObject]@{
            profileType = $ProfileType
            content     = $null
            profilePath = GetProfilePath -profileType $ProfileType
            _exist      = $false
        }

        # Add ToJson method
        $resource | Add-Member -MemberType ScriptMethod -Name 'ToJson' -Value {
            return ([ordered] @{
                    profileType = $this.profileType
                    content     = $this.content
                    profilePath = $this.profilePath
                    _exist      = $this._exist
                }) | ConvertTo-Json -Compress -EnumsAsStrings
        }

        # Constructor logic - if Content and Exist parameters are provided (WithContent parameter set)
        if ($PSCmdlet.ParameterSetName -eq 'WithContent') {
            $resource.content = $Content
            $resource._exist = $Exist
        } else {
            # Default constructor logic - read from file system
            $fileExists = Test-Path $resource.profilePath
            if ($fileExists) {
                $resource.content = Get-Content -Path $resource.profilePath
            } else {
                $resource.content = $null
            }
            $resource._exist = $fileExists
        }

        return $resource
    }

    function GetProfilePath {
        param (
            [ProfileType] $profileType
        )

        $path = switch ($profileType) {
            'AllUsersCurrentHost' { $PROFILE.AllUsersCurrentHost }
            'AllUsersAllHosts' { $PROFILE.AllUsersAllHosts }
            'CurrentUserAllHosts' { $PROFILE.CurrentUserAllHosts }
            'CurrentUserCurrentHost' { $PROFILE.CurrentUserCurrentHost }
        }

        return $path
    }

    function ExportOperation {
        $allUserCurrentHost = New-PwshResource -ProfileType 'AllUsersCurrentHost'
        $allUsersAllHost = New-PwshResource -ProfileType 'AllUsersAllHosts'
        $currentUserAllHost = New-PwshResource -ProfileType 'CurrentUserAllHosts'
        $currentUserCurrentHost = New-PwshResource -ProfileType 'CurrentUserCurrentHost'

        # Cannot use the ToJson() method here as we are adding a note property
        $allUserCurrentHost | Add-Member -NotePropertyName '_name' -NotePropertyValue 'AllUsersCurrentHost' -PassThru | ConvertTo-Json -Compress -EnumsAsStrings
        $allUsersAllHost | Add-Member -NotePropertyName '_name' -NotePropertyValue 'AllUsersAllHosts' -PassThru | ConvertTo-Json -Compress -EnumsAsStrings
        $currentUserAllHost | Add-Member -NotePropertyName '_name' -NotePropertyValue 'CurrentUserAllHosts' -PassThru | ConvertTo-Json -Compress -EnumsAsStrings
        $currentUserCurrentHost | Add-Member -NotePropertyName '_name' -NotePropertyValue 'CurrentUserCurrentHost' -PassThru | ConvertTo-Json -Compress -EnumsAsStrings
    }

    function GetOperation {
        param (
            [Parameter(Mandatory = $true)]
            $InputResource,
            [Parameter()]
            [switch] $AsJson
        )

        $profilePath = GetProfilePath -profileType $InputResource.profileType.ToString()

        $actualState = New-PwshResource -ProfileType $InputResource.profileType

        $actualState.profilePath = $profilePath

        $exists = Test-Path $profilePath

        if ($InputResource._exist -and $exists) {
            $content = Get-Content -Path $profilePath
            $actualState.Content = $content
        } elseif ($InputResource._exist -and -not $exists) {
            $actualState.Content = $null
            $actualState._exist = $false
        } elseif (-not $InputResource._exist -and $exists) {
            $actualState.Content = Get-Content -Path $profilePath
            $actualState._exist = $true
        } else {
            $actualState.Content = $null
            $actualState._exist = $false
        }

        if ($AsJson) {
            return $actualState.ToJson()
        } else {
            return $actualState
        }
    }

    function SetOperation {
        param (
            $InputResource
        )

        $actualState = GetOperation -InputResource $InputResource

        if ($InputResource._exist) {
            if (-not $actualState._exist) {
                $null = New-Item -Path $actualState.profilePath -ItemType File -Force
            }

            if ($null -ne $InputResource.content) {
                Set-Content -Path $actualState.profilePath -Value $InputResource.content
            }
        } elseif ($actualState._exist) {
            Remove-Item -Path $actualState.profilePath -Force
        }
    }
}
End {
    $inputJson = $input | ConvertFrom-Json

    if ($inputJson) {
        $InputResource = New-PwshResource -ProfileType $inputJson.profileType -Content $inputJson.content -Exist $inputJson._exist
    }

    switch ($Operation) {
        'get' {
            GetOperation -InputResource $InputResource -AsJson
        }
        'set' {
            SetOperation -InputResource $InputResource
        }
        'export' {
            if ($inputJson) {
                Write-Error "Input not supported for export operation"
                exit 2
            }

            ExportOperation
        }
    }

    exit 0
}

# SIG # Begin signature block
# MIInOgYJKoZIhvcNAQcCoIInKzCCJycCAQExDzANBglghkgBZQMEAgEFADB5Bgor
# BgEEAYI3AgEEoGswaTA0BgorBgEEAYI3AgEeMCYCAwEAAAQQH8w7YFlLCE63JNLG
# KX7zUQIBAAIBAAIBAAIBAAIBADAxMA0GCWCGSAFlAwQCAQUABCByJLrmewddsBTG
# 7RhmZpu3+7n6ngfUChFqTBK/Ws+SaKCCDMkwggYEMIID7KADAgECAhMzAAACHPrN
# xZvoL37EAAAAAAIcMA0GCSqGSIb3DQEBCwUAMFcxCzAJBgNVBAYTAlVTMR4wHAYD
# VQQKExVNaWNyb3NvZnQgQ29ycG9yYXRpb24xKDAmBgNVBAMTH01pY3Jvc29mdCBD
# b2RlIFNpZ25pbmcgUENBIDIwMjQwHhcNMjYwNDE2MTg1OTQxWhcNMjcwNDE1MTg1
# OTQxWjB0MQswCQYDVQQGEwJVUzETMBEGA1UECBMKV2FzaGluZ3RvbjEQMA4GA1UE
# BxMHUmVkbW9uZDEeMBwGA1UEChMVTWljcm9zb2Z0IENvcnBvcmF0aW9uMR4wHAYD
# VQQDExVNaWNyb3NvZnQgQ29ycG9yYXRpb24wggEiMA0GCSqGSIb3DQEBAQUAA4IB
# DwAwggEKAoIBAQDVsZfgOKmM31HPfoWOoNEiw0SlCiIxUMC0I9NMWbucKOw/e9lP
# oAoehQVu6SG65V4EPzrYsnBnFPNoi4/HoOdjhz1qkrEt4I6tEcxXU6oOeY9zGveC
# /3iBeuhLYxM3M/PkcUoebF+Nednm8OkdSPoDu8imViHPQq/8CQUu0WRR4rE+dMRf
# rpVqfmNi2qWCX94T4MsepijGVkwE//tJg0ryAiYdHT34LSnlG/RSBZmQRGWZ5g8j
# qnKjRParSqMft1gvjuUTVgtWNZfgcLFSK5Wa0myrq8OPcgTGGsRgun+tnSS+IxDT
# xVsAPH1OzvPjwomguByhUe/OcvUN0D5Wmp7xAgMBAAGjggGqMIIBpjAOBgNVHQ8B
# Af8EBAMCB4AwHwYDVR0lBBgwFgYKKwYBBAGCN0wIAQYIKwYBBQUHAwMwHQYDVR0O
# BBYEFNoH7a2YDjOSwpkp6DHcmUS7J+0yMFQGA1UdEQRNMEukSTBHMS0wKwYDVQQL
# EyRNaWNyb3NvZnQgSXJlbGFuZCBPcGVyYXRpb25zIExpbWl0ZWQxFjAUBgNVBAUT
# DTIzMDAxMis1MDc1NjkwHwYDVR0jBBgwFoAUf1k/VCHarU/vBeXmo9ctBpQSCDEw
# YAYDVR0fBFkwVzBVoFOgUYZPaHR0cDovL3d3dy5taWNyb3NvZnQuY29tL3BraW9w
# cy9jcmwvTWljcm9zb2Z0JTIwQ29kZSUyMFNpZ25pbmclMjBQQ0ElMjAyMDI0LmNy
# bDBtBggrBgEFBQcBAQRhMF8wXQYIKwYBBQUHMAKGUWh0dHA6Ly93d3cubWljcm9z
# b2Z0LmNvbS9wa2lvcHMvY2VydHMvTWljcm9zb2Z0JTIwQ29kZSUyMFNpZ25pbmcl
# MjBQQ0ElMjAyMDI0LmNydDAMBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IC
# AQAUnEqhaRXe0T3hIJjvdQErEkrA/7bByjn6t5IArODkkRjzkYwtKMc2yYj2quaN
# rLutWw2YZcngKPy1b71YyDJQTy4NDRwaSh9Tw5thrk3NmcPrAHia5vtcBJ1CgtKK
# 7mQbIcQ22d/N3813ayCDDFewu1+jsZmX+r/aTEqaOM4TVxVtRSkuCy8nAXKuChOK
# Li/zA4XuH8iEYqIsj2YoNaeSxVmeGiERXpKdo3dDmYi0kO5w2D8VS4c3+9h6gElY
# BaAAg/dYErBg27qT3vv0zRDJhJufvCNylA8S7/+8H5E/PV5cng6na9VV/w9OV3qu
# uND6zdGa2EX38Glp50F9AIQk3p2xXmcvorDeM4XJ7UlWYBi6g80J1SSOQnInCYFE
# msfUNn3+1AaTJKSJL83quKArTac2pKhu0Yzzzrzo6HrsRiQKzpnRBb1/dMa6P3hz
# 75XbMRBctNsFhZC07WCmjExdLg2eHW5uV0TY8D5+6wozJf7vF3+WHkYPO85Z+BC6
# U4FkNbYNycZ9cE4j1tXRdyDCfml6c0HWPHjNVDObrv9lKt3qUqFpX38VCqVCyNOO
# 1UcXfQiVjJw32U2WUKZjt/neJKHEBsm9kFsLuWzkQ53+qcaSaytmsCnk2gOglrlD
# 5d3kKyvvAw+rzm0lT8K38P6PLxfZQHhu4W8dV7Av8N2ZmDCCBr0wggSloAMCAQIC
# EzMAAAA5O7Y3Gb8GHWcAAAAAADkwDQYJKoZIhvcNAQEMBQAwgYgxCzAJBgNVBAYT
# AlVTMRMwEQYDVQQIEwpXYXNoaW5ndG9uMRAwDgYDVQQHEwdSZWRtb25kMR4wHAYD
# VQQKExVNaWNyb3NvZnQgQ29ycG9yYXRpb24xMjAwBgNVBAMTKU1pY3Jvc29mdCBS
# b290IENlcnRpZmljYXRlIEF1dGhvcml0eSAyMDExMB4XDTI0MDgwODIwNTQxOFoX
# DTM2MDMyMjIyMTMwNFowVzELMAkGA1UEBhMCVVMxHjAcBgNVBAoTFU1pY3Jvc29m
# dCBDb3Jwb3JhdGlvbjEoMCYGA1UEAxMfTWljcm9zb2Z0IENvZGUgU2lnbmluZyBQ
# Q0EgMjAyNDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBANgBnB7jOMeq
# lRYHNa265v4IY9fH8TKhemHfPINe1gpLaV3dhg324WwH06LcHbpnsBukCDNitryo
# 0dtS/EW6I/yEL/bLSY8hKpbfQuWusBPr9qazYcDxCW/qnjb5JsI1s8bNOg3bVATv
# QVL4tcf03aTycsz8QeCdM0l/yHRObJ9QqazM1r6VPEOJ7LL+uEEb73w6QCuhs89a
# 1uv1zerOYMnsneRRwCbpyW11IcggU0cRKDDq1pjVJzIbIF6+oiXXbReOsgeI8zu1
# FyQfK0fVkaya8SmVHQ/tOf23mZ4W9k0Ri22QW9p3UgSC5OUDktKxxcCmGL6tXLfO
# GSWHIIV4YrTJTT6PNty5REojHJuZHArkF9VnHTERWoTjAzfI3kP+5b4alUdhgAZ7
# ttOu1bVnXfHaqPYl2rPs20ji03LOVWsh/radgE17es5hL+t6lV0eVHrVhsssROWJ
# uz2MXMCt7iw7lFPG9LXKGjsmonn2gotGdHIuEg5JnJMJVmixd5LRlkmgYRZKzhxS
# CwyoGIq0PhaA7Y+VPct5pCHkijcIIDm0nlkK+0KyepolcqGm0T/GYQRMhHJlGOOm
# VQop36wUVUYklUy++vDWeEgEo4s7hxN6mIbf2MSIQ/iIfMZgJxC69oukMUXCrOC3
# SkE/xIkgpfl22MM1itkZ35nNXkMolU1lAgMBAAGjggFOMIIBSjAOBgNVHQ8BAf8E
# BAMCAYYwEAYJKwYBBAGCNxUBBAMCAQAwHQYDVR0OBBYEFH9ZP1Qh2q1P7wXl5qPX
# LQaUEggxMBkGCSsGAQQBgjcUAgQMHgoAUwB1AGIAQwBBMA8GA1UdEwEB/wQFMAMB
# Af8wHwYDVR0jBBgwFoAUci06AjGQQ7kUBU7h6qfHMdEjiTQwWgYDVR0fBFMwUTBP
# oE2gS4ZJaHR0cDovL2NybC5taWNyb3NvZnQuY29tL3BraS9jcmwvcHJvZHVjdHMv
# TWljUm9vQ2VyQXV0MjAxMV8yMDExXzAzXzIyLmNybDBeBggrBgEFBQcBAQRSMFAw
# TgYIKwYBBQUHMAKGQmh0dHA6Ly93d3cubWljcm9zb2Z0LmNvbS9wa2kvY2VydHMv
# TWljUm9vQ2VyQXV0MjAxMV8yMDExXzAzXzIyLmNydDANBgkqhkiG9w0BAQwFAAOC
# AgEAFJQfOChP7onn6fLIMKrSlN1WYKwDFgAddymOUO3FrM8d7B/W/iQ6DxXsDn7D
# 5W4wMwYeLystcEqfkjz4NURRgazyMu5yRzQh4LqjA4tStTcJh1opExo7nn5PuPBY
# nbu0+THSuVHTe0VTTPVhily/piFrDo3axQ9P4C+Ol5yet+2gTfekICS5xS+cYfSI
# vgn0JksVBVMYVI5QFu/qhnLhsEFEUzG8fvv0hjgkO+lkpV9ty6GkN4vdnd7ya6Q6
# aR9y34aiM1qmxaxBi6OUnyNl6fkuun/diTFnYDLTppOkr/mg5WSfCiDVMNCxtj4w
# PKC5OmHm1DQIt/MNokbbH3UGsFP1QbzsLocuSqLCvH09Io3fDPTmscR9Y75G4qX7
# RTX8AdBPo0I6OEojf39zuFZt0qOHm65YWQE69cZM2ueE1MB05dNNgHK9gTE7zKvK
# /fg8B2qjW88MT/WF5V5uvZGtqa9FSL2RazArA+rDPuf6JGYz4HpgMZHB4S6szWSK
# YBv0VisCzfxgeU+dquXW9bd0auYlOB58DPcOYKdc3Se94g+xL4pcEhbB54JOgAkw
# YTu/9dLeH2pDqeJZAABVDWRQCaXfO5LgyKwKCLYXpigrZYCjUSBcr+Ve8PFWMhVT
# Ql0v4q8J/AUmQN5W4n101cY2L4A7GTQG1h32HHAvfQESWP0xghnHMIIZwwIBATBu
# MFcxCzAJBgNVBAYTAlVTMR4wHAYDVQQKExVNaWNyb3NvZnQgQ29ycG9yYXRpb24x
# KDAmBgNVBAMTH01pY3Jvc29mdCBDb2RlIFNpZ25pbmcgUENBIDIwMjQCEzMAAAIc
# +s3Fm+gvfsQAAAAAAhwwDQYJYIZIAWUDBAIBBQCggZAwGQYJKoZIhvcNAQkDMQwG
# CisGAQQBgjcCAQQwLwYJKoZIhvcNAQkEMSIEIJpzkwdoxpb2+tSsoFjmFoiV37jR
# JCGrYb29mF3i7GhyMEIGCisGAQQBgjcCAQwxNDAyoBSAEgBNAGkAYwByAG8AcwBv
# AGYAdKEagBhodHRwOi8vd3d3Lm1pY3Jvc29mdC5jb20wDQYJKoZIhvcNAQEBBQAE
# ggEAdzpllE75rhMRV+S6XDvEOpFHehlWBdyPqSnhRNh/E3e2a+LbHSxAy2FpNVdV
# hVcguWuZoX2tZQ0gSuQkmzcw0kuQ89REg+Vfamv8RtF6LVP9NY4mqBoiLBBkPNws
# no2Rpo/keum3wJDT8vuE/gvIT2nJPyf4Fv2wElFHKImKkixId4lFmol/xuelIesl
# xqUT6HtnCGrptO8uGDsvEVUkQ9pKTXJXDADwFdLMtUJovqfDn4Fo7UraS+hO7hyd
# mLi1RpppEcF+UIUYPztH6Hz962oE5w1EWp/ngg/k+EFb/alj4WCeaXNQ435tsfUt
# 0hviGriKf0lh54T7Lse8hhH3EqGCF5cwgheTBgorBgEEAYI3AwMBMYIXgzCCF38G
# CSqGSIb3DQEHAqCCF3AwghdsAgEDMQ8wDQYJYIZIAWUDBAIBBQAwggFSBgsqhkiG
# 9w0BCRABBKCCAUEEggE9MIIBOQIBAQYKKwYBBAGEWQoDATAxMA0GCWCGSAFlAwQC
# AQUABCBezUVlRADWWRQBAwPMVPiV4wZDve7oRLfngltAowHyXgIGal/FQep0GBMy
# MDI2MDgxMTE2MjIxNi45NTZaMASAAgH0oIHRpIHOMIHLMQswCQYDVQQGEwJVUzET
# MBEGA1UECBMKV2FzaGluZ3RvbjEQMA4GA1UEBxMHUmVkbW9uZDEeMBwGA1UEChMV
# TWljcm9zb2Z0IENvcnBvcmF0aW9uMSUwIwYDVQQLExxNaWNyb3NvZnQgQW1lcmlj
# YSBPcGVyYXRpb25zMScwJQYDVQQLEx5uU2hpZWxkIFRTUyBFU046ODYwMy0wNUUw
# LUQ5NDcxJTAjBgNVBAMTHE1pY3Jvc29mdCBUaW1lLVN0YW1wIFNlcnZpY2WgghHt
# MIIHIDCCBQigAwIBAgITMwAAAiWAxzfGzap3SQABAAACJTANBgkqhkiG9w0BAQsF
# ADB8MQswCQYDVQQGEwJVUzETMBEGA1UECBMKV2FzaGluZ3RvbjEQMA4GA1UEBxMH
# UmVkbW9uZDEeMBwGA1UEChMVTWljcm9zb2Z0IENvcnBvcmF0aW9uMSYwJAYDVQQD
# Ex1NaWNyb3NvZnQgVGltZS1TdGFtcCBQQ0EgMjAxMDAeFw0yNjAyMTkxOTQwMDFa
# Fw0yNzA1MTcxOTQwMDFaMIHLMQswCQYDVQQGEwJVUzETMBEGA1UECBMKV2FzaGlu
# Z3RvbjEQMA4GA1UEBxMHUmVkbW9uZDEeMBwGA1UEChMVTWljcm9zb2Z0IENvcnBv
# cmF0aW9uMSUwIwYDVQQLExxNaWNyb3NvZnQgQW1lcmljYSBPcGVyYXRpb25zMScw
# JQYDVQQLEx5uU2hpZWxkIFRTUyBFU046ODYwMy0wNUUwLUQ5NDcxJTAjBgNVBAMT
# HE1pY3Jvc29mdCBUaW1lLVN0YW1wIFNlcnZpY2UwggIiMA0GCSqGSIb3DQEBAQUA
# A4ICDwAwggIKAoICAQCm8RIP0eLA46VcCPovvmqsIlN6qkmz5IsHWmUU0neUqp8u
# Gxadeo+SwWBCwQ5alZI/DNdpXfyiZLZR6XYgpRPFzepIl7OCDb4NtEskJCIZDkQM
# NwrH9YwUyu71GGigsLIxeleHtA3utoVTeHjS1b8UnwORRtknKkyrUArT6ZpB2rod
# IcmcLcv3x3wwgYlOs0FEg5EsVrZb7LNc/nd0bXDp+HTOWWui8eoTVwJeLxcVP869
# oF8li5SU81aa2tGJ6/Jsejiz9JMW8SJXKBT2DCXMOUkCsGjonPZRqfvoMSIQZgta
# OTyAJlrvsy0TZ78XrGqoygtQimQnbOAL4KNLSCuW5TZEQGTHLOQJGgggb3j5gKC7
# 78+RIPJA+n/hmHJ/x4qT/HTTPoVeMCcuBKWrQXR1+/pYau3Fwe0tWIyG+LWzkRr/
# ZNPPupcA2Yci3qn8HR9RwvQopqSNJwn2Ri6am8AQyfVVy/BBw0t6jpoRPjwKvuUj
# fCzpae6duOxQtQ1XDN9PA2yl9sDko/+AXV/SOe8ea8QoQcv3s3ErkG+Lp6hnvw6O
# MPian4ggNkRtgtB7ro1OiopOUXJn9Y5EO3JUAXNcuM9m+5My1VEuvGytgAH3uxms
# lTnW3YbrfazaySCSSnWkhaOZ33hgbuUQfH7n2NFEAUc/cFzfmCQUikWisnJYywID
# AQABo4IBSTCCAUUwHQYDVR0OBBYEFLE40qoXTuMHX3AfZUu1n8nx2h93MB8GA1Ud
# IwQYMBaAFJ+nFV0AXmJdg/Tl0mWnG1M1GelyMF8GA1UdHwRYMFYwVKBSoFCGTmh0
# dHA6Ly93d3cubWljcm9zb2Z0LmNvbS9wa2lvcHMvY3JsL01pY3Jvc29mdCUyMFRp
# bWUtU3RhbXAlMjBQQ0ElMjAyMDEwKDEpLmNybDBsBggrBgEFBQcBAQRgMF4wXAYI
# KwYBBQUHMAKGUGh0dHA6Ly93d3cubWljcm9zb2Z0LmNvbS9wa2lvcHMvY2VydHMv
# TWljcm9zb2Z0JTIwVGltZS1TdGFtcCUyMFBDQSUyMDIwMTAoMSkuY3J0MAwGA1Ud
# EwEB/wQCMAAwFgYDVR0lAQH/BAwwCgYIKwYBBQUHAwgwDgYDVR0PAQH/BAQDAgeA
# MA0GCSqGSIb3DQEBCwUAA4ICAQAHnfc2yUyoHZbvvyVKFuXh5HxxHIvIaR9JWpIf
# ITJlc/Ki03juR+vckzq3tp5fFH5LL7eIFXRIuoewMsvWeFrWufrrW4HhmhCwkqAr
# fA1C0xk+HaYs2O48YSxMX9lgS1kTTIb3YsfoFdFpKurPf2nc2Yd4wLg+Fgwmkxke
# yE3MUKVna8SZeVpEjnS5ucFck4srPwK2ORAf70I23GGyPhqgIKZphNXhSscTAQsy
# IqB5GwDMdRV5LK37NfU4YmxvCYh3TFYE/Gh01Q6yJvf9HxiEZpwW+oUk0gruHobg
# 3sgIR5rfgUo8l30vUnaDYMcPAClaFMC/QbHZSaUhWXZG1OOcMp0g9vYQNLDEqFX2
# jlquvzVSSwtHtm1KTldCjRED+kdCybcPxbPalwJigXc1BsI9CitnTf0ljwb9NkZ/
# JVI8/D62rXXzhz4F3u0iVGzwncGaxRxHG/Xv4nTrpkOeepoYbNBbMWS2G1qP3Xj7
# pVf0+4qRyAqJ0stjQjoVOJImVPWRjz5PR3Dn6adQVMBJDM6gDrj1rZTFVgCtTijq
# GZSGzvXpGkF3vYsyE6ZDma/kGdiUe5saeI6lH66PiWWXgqxt7sy2Ezv0yIjSVv+e
# MOT2QMUiZ6WCc7gVtAmXpfeIus+NmgFvM+Ic1X58e4I9EL4ZSAidSpWW0GZTLNC0
# 2mryLjCCB3EwggVZoAMCAQICEzMAAAAVxedrngKbSZkAAAAAABUwDQYJKoZIhvcN
# AQELBQAwgYgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIEwpXYXNoaW5ndG9uMRAwDgYD
# VQQHEwdSZWRtb25kMR4wHAYDVQQKExVNaWNyb3NvZnQgQ29ycG9yYXRpb24xMjAw
# BgNVBAMTKU1pY3Jvc29mdCBSb290IENlcnRpZmljYXRlIEF1dGhvcml0eSAyMDEw
# MB4XDTIxMDkzMDE4MjIyNVoXDTMwMDkzMDE4MzIyNVowfDELMAkGA1UEBhMCVVMx
# EzARBgNVBAgTCldhc2hpbmd0b24xEDAOBgNVBAcTB1JlZG1vbmQxHjAcBgNVBAoT
# FU1pY3Jvc29mdCBDb3Jwb3JhdGlvbjEmMCQGA1UEAxMdTWljcm9zb2Z0IFRpbWUt
# U3RhbXAgUENBIDIwMTAwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDk
# 4aZM57RyIQt5osvXJHm9DtWC0/3unAcH0qlsTnXIyjVX9gF/bErg4r25PhdgM/9c
# T8dm95VTcVrifkpa/rg2Z4VGIwy1jRPPdzLAEBjoYH1qUoNEt6aORmsHFPPFdvWG
# UNzBRMhxXFExN6AKOG6N7dcP2CZTfDlhAnrEqv1yaa8dq6z2Nr41JmTamDu6Gnsz
# rYBbfowQHJ1S/rboYiXcag/PXfT+jlPP1uyFVk3v3byNpOORj7I5LFGc6XBpDco2
# LXCOMcg1KL3jtIckw+DJj361VI/c+gVVmG1oO5pGve2krnopN6zL64NF50ZuyjLV
# wIYwXE8s4mKyzbnijYjklqwBSru+cakXW2dg3viSkR4dPf0gz3N9QZpGdc3EXzTd
# EonW/aUgfX782Z5F37ZyL9t9X4C626p+Nuw2TPYrbqgSUei/BQOj0XOmTTd0lBw0
# gg/wEPK3Rxjtp+iZfD9M269ewvPV2HM9Q07BMzlMjgK8QmguEOqEUUbi0b1qGFph
# AXPKZ6Je1yh2AuIzGHLXpyDwwvoSCtdjbwzJNmSLW6CmgyFdXzB0kZSU2LlQ+QuJ
# YfM2BjUYhEfb3BvR/bLUHMVr9lxSUV0S2yW6r1AFemzFER1y7435UsSFF5PAPBXb
# GjfHCBUYP3irRbb1Hode2o+eFnJpxq57t7c+auIurQIDAQABo4IB3TCCAdkwEgYJ
# KwYBBAGCNxUBBAUCAwEAATAjBgkrBgEEAYI3FQIEFgQUKqdS/mTEmr6CkTxGNSnP
# EP8vBO4wHQYDVR0OBBYEFJ+nFV0AXmJdg/Tl0mWnG1M1GelyMFwGA1UdIARVMFMw
# UQYMKwYBBAGCN0yDfQEBMEEwPwYIKwYBBQUHAgEWM2h0dHA6Ly93d3cubWljcm9z
# b2Z0LmNvbS9wa2lvcHMvRG9jcy9SZXBvc2l0b3J5Lmh0bTATBgNVHSUEDDAKBggr
# BgEFBQcDCDAZBgkrBgEEAYI3FAIEDB4KAFMAdQBiAEMAQTALBgNVHQ8EBAMCAYYw
# DwYDVR0TAQH/BAUwAwEB/zAfBgNVHSMEGDAWgBTV9lbLj+iiXGJo0T2UkFvXzpoY
# xDBWBgNVHR8ETzBNMEugSaBHhkVodHRwOi8vY3JsLm1pY3Jvc29mdC5jb20vcGtp
# L2NybC9wcm9kdWN0cy9NaWNSb29DZXJBdXRfMjAxMC0wNi0yMy5jcmwwWgYIKwYB
# BQUHAQEETjBMMEoGCCsGAQUFBzAChj5odHRwOi8vd3d3Lm1pY3Jvc29mdC5jb20v
# cGtpL2NlcnRzL01pY1Jvb0NlckF1dF8yMDEwLTA2LTIzLmNydDANBgkqhkiG9w0B
# AQsFAAOCAgEAnVV9/Cqt4SwfZwExJFvhnnJL/Klv6lwUtj5OR2R4sQaTlz0xM7U5
# 18JxNj/aZGx80HU5bbsPMeTCj/ts0aGUGCLu6WZnOlNN3Zi6th542DYunKmCVgAD
# sAW+iehp4LoJ7nvfam++Kctu2D9IdQHZGN5tggz1bSNU5HhTdSRXud2f8449xvNo
# 32X2pFaq95W2KFUn0CS9QKC/GbYSEhFdPSfgQJY4rPf5KYnDvBewVIVCs/wMnosZ
# iefwC2qBwoEZQhlSdYo2wh3DYXMuLGt7bj8sCXgU6ZGyqVvfSaN0DLzskYDSPeZK
# PmY7T7uG+jIa2Zb0j/aRAfbOxnT99kxybxCrdTDFNLB62FD+CljdQDzHVG2dY3RI
# LLFORy3BFARxv2T5JL5zbcqOCb2zAVdJVGTZc9d/HltEAY5aGZFrDZ+kKNxnGSgk
# ujhLmm77IVRrakURR6nxt67I6IleT53S0Ex2tVdUCbFpAUR+fKFhbHP+CrvsQWY9
# af3LwUFJfn6Tvsv4O+S3Fb+0zj6lMVGEvL8CwYKiexcdFYmNcP7ntdAoGokLjzba
# ukz5m/8K6TT4JDVnK+ANuOaMmdbhIurwJ0I9JZTmdHRbatGePu1+oDEzfbzL6Xu/
# OHBE0ZDxyKs6ijoIYn/ZcGNTTY3ugm2lBRDBcQZqELQdVTNYs6FwZvKhggNQMIIC
# OAIBATCB+aGB0aSBzjCByzELMAkGA1UEBhMCVVMxEzARBgNVBAgTCldhc2hpbmd0
# b24xEDAOBgNVBAcTB1JlZG1vbmQxHjAcBgNVBAoTFU1pY3Jvc29mdCBDb3Jwb3Jh
# dGlvbjElMCMGA1UECxMcTWljcm9zb2Z0IEFtZXJpY2EgT3BlcmF0aW9uczEnMCUG
# A1UECxMeblNoaWVsZCBUU1MgRVNOOjg2MDMtMDVFMC1EOTQ3MSUwIwYDVQQDExxN
# aWNyb3NvZnQgVGltZS1TdGFtcCBTZXJ2aWNloiMKAQEwBwYFKw4DAhoDFQBTb+bK
# OPAjCBflhzw5EXBuSWxeDqCBgzCBgKR+MHwxCzAJBgNVBAYTAlVTMRMwEQYDVQQI
# EwpXYXNoaW5ndG9uMRAwDgYDVQQHEwdSZWRtb25kMR4wHAYDVQQKExVNaWNyb3Nv
# ZnQgQ29ycG9yYXRpb24xJjAkBgNVBAMTHU1pY3Jvc29mdCBUaW1lLVN0YW1wIFBD
# QSAyMDEwMA0GCSqGSIb3DQEBCwUAAgUA7iVHkTAiGA8yMDI2MDgxMTA3MDI0MVoY
# DzIwMjYwODEyMDcwMjQxWjB3MD0GCisGAQQBhFkKBAExLzAtMAoCBQDuJUeRAgEA
# MAoCAQACAhOlAgH/MAcCAQACAhIrMAoCBQDuJpkRAgEAMDYGCisGAQQBhFkKBAIx
# KDAmMAwGCisGAQQBhFkKAwKgCjAIAgEAAgMHoSChCjAIAgEAAgMBhqAwDQYJKoZI
# hvcNAQELBQADggEBABp3KG7yXBV+vyQjrDxq73BBOPHc8XRVY1JyTm7rryfTKeqb
# t9TAuLegPBLw5KyFv7qoguSpqu6d5MqsTwN5UFmgYseWJYmRvhAyqQHsv//Nuifu
# PhSfGeJvVPt7WsBm2c6s9KK0k2HDi4mrJVJGeVjVUyd48tjzi/TaBL5BbaVD4KGB
# PLJtcjgQVk7ob27c8knTguGTEkuWfkWhptxbrckE5cjfesEH+N146KxyAqf7/HHh
# 2fX3JJ355ekjRo6GLUV5LJDqPpitwnuXFbyD5XRQJDgKKQNADiYmz2DQbDROeHCk
# lPsnBJqg0sG4oZ/u5Iwt/4CEap67H2lC/yK8h1UxggQNMIIECQIBATCBkzB8MQsw
# CQYDVQQGEwJVUzETMBEGA1UECBMKV2FzaGluZ3RvbjEQMA4GA1UEBxMHUmVkbW9u
# ZDEeMBwGA1UEChMVTWljcm9zb2Z0IENvcnBvcmF0aW9uMSYwJAYDVQQDEx1NaWNy
# b3NvZnQgVGltZS1TdGFtcCBQQ0EgMjAxMAITMwAAAiWAxzfGzap3SQABAAACJTAN
# BglghkgBZQMEAgEFAKCCAUowGgYJKoZIhvcNAQkDMQ0GCyqGSIb3DQEJEAEEMC8G
# CSqGSIb3DQEJBDEiBCCTqDDIlyx5bjsFvyWZvOCKtGyIYttjd1xDNiSiepQfuzCB
# +gYLKoZIhvcNAQkQAi8xgeowgecwgeQwgb0EIFYN7oh6ON3y92CmAl/lF0CYwrjW
# WQP6dCUxajPSHKEQMIGYMIGApH4wfDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCldh
# c2hpbmd0b24xEDAOBgNVBAcTB1JlZG1vbmQxHjAcBgNVBAoTFU1pY3Jvc29mdCBD
# b3Jwb3JhdGlvbjEmMCQGA1UEAxMdTWljcm9zb2Z0IFRpbWUtU3RhbXAgUENBIDIw
# MTACEzMAAAIlgMc3xs2qd0kAAQAAAiUwIgQgYg3mMTzHy1AuV7/pcVdjHP6xW9Sv
# EFfecftQdQWUcrAwDQYJKoZIhvcNAQELBQAEggIAPBtVgdYiZoUdkkofWlpD/74b
# 4Buz9rGy6mZ9Gw8D67KcyBzjsXSIolXFOI2938RevCaZ4yfmjZHE6YDhLU6KujHj
# 3iMU4QY7//i6UjxqrbyXT/ejnjkH8PaDFeOrIP0Ub2CIA+SV68bhFuzvL2ojhBIi
# nbi07/H/HLiry7ycPpEN3zXSHrLvCJY5mt+pRkiQ8IrXxzALRTjc8OE9LleuP6Ij
# +uhbFagSh37eVIPzcna7SrlIAKXGpgttu/WDIGaORm4OkoquWOeoFIoDa/qIY3cP
# W09gMrpvMjllEF/K5oJJ1tdjoHKzu+r4w8Tr/3WYGTLG1VCCwa6zsKkKDIQx2b78
# 8sai3jqObROniCI6cTuriohS9hWFcRgGTD6CRXRphj4Wehsf0+0epG5XgIyo9/mB
# 2VtBzMlQ1ykFao/9YjOPVau+vpIxpkkZCJkCqeypz6OYWm3xO9lkhPWeyJ6xJ7ro
# aFqti6DY3p8IruScOU0VqYZT6sAl5WNTB7jSQyBu/XA3wJpF12wLTJPvRcxjEhS1
# shsBK2wRDhOPfW01MaYgMzt+2uT4xtN3Xp/khJ/aZ9qoGs7soKhpAZoZrVmR37H1
# ZQnzxmlzvM7FYx/m1wm/iq9iCuOHyg6cI3+svdBCrFJxPvuw32Kx1eaSWhl3d3hC
# Nr5tOk8UG30sAZJWWtY=
# SIG # End signature block
