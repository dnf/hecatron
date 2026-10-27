BeforeAll {
    Import-Module (Join-Path $PSScriptRoot '..' 'Add.psd1') -Force
}

Describe 'Add-Number' {
    It 'adds two numbers' {
        Add-Number -A 2 -B 3 | Should -Be 5
    }
}
