function Add-Number {
    <#
    .SYNOPSIS
    Adds two numbers.
    #>
    [CmdletBinding()]
    [OutputType([int])]
    param(
        [Parameter(Mandatory)] [int] $A,
        [Parameter(Mandatory)] [int] $B
    )
    $A + $B
}

Export-ModuleMember -Function Add-Number
