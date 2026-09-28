[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Source,

    [Parameter(Mandatory = $true)]
    [string]$PackRoot,

    [switch]$SourceIsOptical,

    [switch]$WindowsOnly
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$sourcePath = (Resolve-Path -LiteralPath $Source).Path
$packPath = [IO.Path]::GetFullPath($PackRoot)
$ffmpeg = (Get-Command ffmpeg -ErrorAction Stop).Source
$temporarySourcePath = $null

$packPrefix = $packPath.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
if ($sourcePath.StartsWith($packPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    $temporarySourcePath = Join-Path ([IO.Path]::GetTempPath()) ("kwu-icon-source-{0}{1}" -f [Guid]::NewGuid(), [IO.Path]::GetExtension($sourcePath))
    Copy-Item -LiteralPath $sourcePath -Destination $temporarySourcePath -Force
    $sourcePath = $temporarySourcePath
}

function Ensure-Directory([string]$Path) {
    [IO.Directory]::CreateDirectory($Path) | Out-Null
}

function Reset-ManagedDirectory([string]$Path) {
    if ([IO.Directory]::Exists($Path)) {
        [IO.Directory]::Delete($Path, $true)
    }
    Ensure-Directory $Path
}

function Invoke-FFmpeg([string[]]$Arguments) {
    & $ffmpeg @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "ffmpeg failed with exit code $LASTEXITCODE"
    }
}

function Save-Scaled([string]$InputPath, [int]$Size, [string]$OutputPath) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    Invoke-FFmpeg @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", $InputPath,
        "-vf", "scale=${Size}:${Size}:flags=lanczos",
        "-frames:v", "1",
        $OutputPath
    )
}

function Save-OpticalFrame([string]$InputPath, [int]$Size, [string]$OutputPath) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    Invoke-FFmpeg @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", $InputPath,
        "-vf", "crop=940:940:42:42,scale=${Size}:${Size}:flags=lanczos",
        "-frames:v", "1",
        $OutputPath
    )
}

function New-WindowsTaskbarMaster([string]$InputPath, [string]$OutputPath) {
    $sourceFile = [Drawing.Bitmap]::FromFile($InputPath)
    $source = [Drawing.Bitmap]::new($sourceFile)
    $sourceFile.Dispose()
    $canvas = [Drawing.Bitmap]::new(1024, 1024, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [Drawing.Graphics]::FromImage($canvas)
    $clip = [Drawing.Drawing2D.GraphicsPath]::new()
    try {
        $graphics.Clear([Drawing.Color]::Transparent)
        $graphics.CompositingMode = [Drawing.Drawing2D.CompositingMode]::SourceCopy
        $graphics.CompositingQuality = [Drawing.Drawing2D.CompositingQuality]::HighQuality
        $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $graphics.PixelOffsetMode = [Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $graphics.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::AntiAlias

        $radius = 210
        $diameter = $radius * 2
        $clip.AddArc(0, 0, $diameter, $diameter, 180, 90)
        $clip.AddArc(1024 - $diameter, 0, $diameter, $diameter, 270, 90)
        $clip.AddArc(1024 - $diameter, 1024 - $diameter, $diameter, $diameter, 0, 90)
        $clip.AddArc(0, 1024 - $diameter, $diameter, $diameter, 90, 90)
        $clip.CloseFigure()
        $graphics.SetClip($clip)

        # Windows taskbar renders the pearl-white tile against an almost identical
        # light background. A small optical enlargement gives the coloured cloud
        # enough weight without changing the desktop/macOS master artwork.
        $opticalSize = 1128
        $offset = [int]((1024 - $opticalSize) / 2)
        $graphics.DrawImage(
            $source,
            [Drawing.Rectangle]::new($offset, $offset, $opticalSize, $opticalSize),
            [Drawing.Rectangle]::new(0, 0, $source.Width, $source.Height),
            [Drawing.GraphicsUnit]::Pixel
        )
        $border = [Drawing.Pen]::new([Drawing.Color]::FromArgb(255, 175, 185, 202), 34)
        try {
            $border.Alignment = [Drawing.Drawing2D.PenAlignment]::Inset
            $graphics.CompositingMode = [Drawing.Drawing2D.CompositingMode]::SourceOver
            $graphics.DrawPath($border, $clip)
        }
        finally {
            $border.Dispose()
        }
        Save-Png $canvas $OutputPath
    }
    finally {
        $clip.Dispose()
        $graphics.Dispose()
        $canvas.Dispose()
        $source.Dispose()
    }
}

function Write-Ico([string]$OutputPath, [object[]]$Frames) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    $items = foreach ($frame in $Frames) {
        [pscustomobject]@{
            Size = [int]$frame.Size
            Data = [IO.File]::ReadAllBytes([string]$frame.Path)
        }
    }

    $stream = [IO.File]::Create($OutputPath)
    $writer = [IO.BinaryWriter]::new($stream)
    try {
        $writer.Write([uint16]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]$items.Count)
        $offset = 6 + (16 * $items.Count)
        foreach ($item in $items) {
            $dimension = if ($item.Size -eq 256) { 0 } else { $item.Size }
            $writer.Write([byte]$dimension)
            $writer.Write([byte]$dimension)
            $writer.Write([byte]0)
            $writer.Write([byte]0)
            $writer.Write([uint16]1)
            $writer.Write([uint16]32)
            $writer.Write([uint32]$item.Data.Length)
            $writer.Write([uint32]$offset)
            $offset += $item.Data.Length
        }
        foreach ($item in $items) {
            $writer.Write($item.Data)
        }
    }
    finally {
        $writer.Dispose()
        $stream.Dispose()
    }
}

function Write-Ascii([IO.BinaryWriter]$Writer, [string]$Value) {
    $Writer.Write([Text.Encoding]::ASCII.GetBytes($Value))
}

function Write-BigEndianUInt32([IO.BinaryWriter]$Writer, [uint32]$Value) {
    $Writer.Write([byte](($Value -shr 24) -band 0xff))
    $Writer.Write([byte](($Value -shr 16) -band 0xff))
    $Writer.Write([byte](($Value -shr 8) -band 0xff))
    $Writer.Write([byte]($Value -band 0xff))
}

function Write-Icns([string]$OutputPath, [object[]]$Entries) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    $items = foreach ($entry in $Entries) {
        $data = [IO.File]::ReadAllBytes([string]$entry.Path)
        [pscustomobject]@{
            Type = [string]$entry.Type
            Data = $data
            Length = 8 + $data.Length
        }
    }
    $tocLength = 8 + (8 * $items.Count)
    $totalLength = 8 + $tocLength + (($items | Measure-Object -Property Length -Sum).Sum)

    $stream = [IO.File]::Create($OutputPath)
    $writer = [IO.BinaryWriter]::new($stream)
    try {
        Write-Ascii $writer "icns"
        Write-BigEndianUInt32 $writer ([uint32]$totalLength)
        Write-Ascii $writer "TOC "
        Write-BigEndianUInt32 $writer ([uint32]$tocLength)
        foreach ($item in $items) {
            Write-Ascii $writer $item.Type
            Write-BigEndianUInt32 $writer ([uint32]$item.Length)
        }
        foreach ($item in $items) {
            Write-Ascii $writer $item.Type
            Write-BigEndianUInt32 $writer ([uint32]$item.Length)
            $writer.Write($item.Data)
        }
    }
    finally {
        $writer.Dispose()
        $stream.Dispose()
    }
}

function Write-SvgWrapper([string]$PngPath, [string]$OutputPath) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    $encoded = [Convert]::ToBase64String([IO.File]::ReadAllBytes($PngPath))
    $svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" role="img" aria-label="云智企办"><title>云智企办</title><image width="1024" height="1024" href="data:image/png;base64,' + $encoded + '"/></svg>'
    [IO.File]::WriteAllText($OutputPath, $svg, [Text.UTF8Encoding]::new($false))
}

function Save-Png([Drawing.Bitmap]$Bitmap, [string]$OutputPath) {
    Ensure-Directory ([IO.Path]::GetDirectoryName($OutputPath))
    $Bitmap.Save($OutputPath, [Drawing.Imaging.ImageFormat]::Png)
}

function New-Wordmarks([string]$SourceWordmark, [string]$SymbolPath, [string]$OnLightPath, [string]$OnDarkPath) {
    $sourceFile = [Drawing.Bitmap]::FromFile($SourceWordmark)
    $source = [Drawing.Bitmap]::new($sourceFile)
    $sourceFile.Dispose()
    $symbolFile = [Drawing.Bitmap]::FromFile($SymbolPath)
    $symbol = [Drawing.Bitmap]::new($symbolFile)
    $symbolFile.Dispose()
    try {
        $dark = [Drawing.Bitmap]::new(422, 135, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
        $graphics = [Drawing.Graphics]::FromImage($dark)
        try {
            $graphics.CompositingMode = [Drawing.Drawing2D.CompositingMode]::SourceCopy
            $sourceRect = if ($source.Width -eq 422 -and $source.Height -eq 135) {
                [Drawing.Rectangle]::new(0, 0, 422, 135)
            }
            else {
                [Drawing.Rectangle]::new(9, 12, 422, 135)
            }
            $graphics.DrawImage($source, [Drawing.Rectangle]::new(0, 0, 422, 135), $sourceRect, [Drawing.GraphicsUnit]::Pixel)
            $graphics.FillRectangle([Drawing.Brushes]::Transparent, 16, 16, 103, 103)
            $graphics.CompositingMode = [Drawing.Drawing2D.CompositingMode]::SourceOver
            $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            $graphics.DrawImage($symbol, 16, 16, 103, 103)
        }
        finally {
            $graphics.Dispose()
        }

        $light = [Drawing.Bitmap]::new($dark)
        for ($y = 0; $y -lt $light.Height; $y++) {
            for ($x = 140; $x -lt $light.Width; $x++) {
                $pixel = $light.GetPixel($x, $y)
                if ($pixel.A -gt 0) {
                    $light.SetPixel($x, $y, [Drawing.Color]::FromArgb($pixel.A, 23, 23, 23))
                }
            }
        }
        try {
            Save-Png $light $OnLightPath
            Save-Png $dark $OnDarkPath
        }
        finally {
            $light.Dispose()
            $dark.Dispose()
        }
    }
    finally {
        $source.Dispose()
        $symbol.Dispose()
    }
}

function New-FullSizePreview([string]$OutputPath, [string]$WordmarkPath, [string]$WindowsPngPath, [int[]]$Sizes) {
    $canvas = [Drawing.Bitmap]::new(1600, 1060, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [Drawing.Graphics]::FromImage($canvas)
    $graphics.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.TextRenderingHint = [Drawing.Text.TextRenderingHint]::ClearTypeGridFit
    $font = [Drawing.Font]::new("Microsoft YaHei UI", 17, [Drawing.FontStyle]::Regular)
    $titleFont = [Drawing.Font]::new("Microsoft YaHei UI", 28, [Drawing.FontStyle]::Bold)
    $panelTitleFont = [Drawing.Font]::new("Microsoft YaHei UI", 21, [Drawing.FontStyle]::Bold)
    $darkBrush = [Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(31, 34, 42))
    $lightBrush = [Drawing.SolidBrush]::new([Drawing.Color]::White)
    try {
        $graphics.Clear([Drawing.Color]::FromArgb(238, 241, 245))
        $graphics.DrawString("云智企办 · Windows 全尺寸图标预览", $titleFont, $darkBrush, 48, 28)
        $panels = @(
            [pscustomobject]@{ Y = 92; Background = [Drawing.Color]::White; Foreground = [Drawing.Color]::FromArgb(31, 34, 42); Title = "浅色背景" },
            [pscustomobject]@{ Y = 576; Background = [Drawing.Color]::FromArgb(31, 34, 42); Foreground = [Drawing.Color]::White; Title = "深色背景" }
        )
        foreach ($panel in $panels) {
            $background = [Drawing.SolidBrush]::new($panel.Background)
            $foreground = [Drawing.SolidBrush]::new($panel.Foreground)
            try {
                $graphics.FillRectangle($background, 30, $panel.Y, 1540, 454)
                $graphics.DrawString($panel.Title, $panelTitleFont, $foreground, 58, $panel.Y + 22)
                for ($index = 0; $index -lt $Sizes.Count; $index++) {
                    $row = [Math]::Floor($index / 7)
                    $column = $index % 7
                    $x = 62 + ($column * 214)
                    $y = $panel.Y + 82 + ($row * 174)
                    $imageFile = [Drawing.Bitmap]::FromFile((Join-Path $WindowsPngPath ("{0}x{0}.png" -f $Sizes[$index])))
                    try {
                        $graphics.DrawImage($imageFile, $x + 55, $y, 96, 96)
                    }
                    finally {
                        $imageFile.Dispose()
                    }
                    $label = "{0} px" -f $Sizes[$index]
                    $graphics.DrawString($label, $font, $foreground, $x + 67, $y + 108)
                }
            }
            finally {
                $background.Dispose()
                $foreground.Dispose()
            }
        }
        Save-Png $canvas $OutputPath
    }
    finally {
        $darkBrush.Dispose()
        $lightBrush.Dispose()
        $font.Dispose()
        $titleFont.Dispose()
        $panelTitleFont.Dispose()
        $graphics.Dispose()
        $canvas.Dispose()
    }
}

function New-ScenePreview([string]$OutputPath, [string]$WindowsPngPath, [string]$OnLightWordmark, [string]$OnDarkWordmark) {
    $canvas = [Drawing.Bitmap]::new(1600, 900, [Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [Drawing.Graphics]::FromImage($canvas)
    $graphics.SmoothingMode = [Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.TextRenderingHint = [Drawing.Text.TextRenderingHint]::ClearTypeGridFit
    $titleFont = [Drawing.Font]::new("Microsoft YaHei UI", 28, [Drawing.FontStyle]::Bold)
    $headingFont = [Drawing.Font]::new("Microsoft YaHei UI", 19, [Drawing.FontStyle]::Bold)
    $bodyFont = [Drawing.Font]::new("Microsoft YaHei UI", 16, [Drawing.FontStyle]::Regular)
    $dark = [Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(31, 34, 42))
    $white = [Drawing.SolidBrush]::new([Drawing.Color]::White)
    $muted = [Drawing.SolidBrush]::new([Drawing.Color]::FromArgb(91, 99, 112))
    try {
        $graphics.Clear([Drawing.Color]::FromArgb(238, 241, 245))
        $graphics.DrawString("云智企办 · 不同场景图标预览", $titleFont, $dark, 48, 28)
        $graphics.FillRectangle([Drawing.Brushes]::White, 46, 96, 430, 350)
        $desktop = [Drawing.Bitmap]::FromFile((Join-Path $WindowsPngPath "256x256.png"))
        try { $graphics.DrawImage($desktop, 141, 122, 240, 240) } finally { $desktop.Dispose() }
        $graphics.DrawString("桌面 / Dock / 应用菜单", $headingFont, $dark, 115, 378)
        $graphics.DrawString("256 px", $bodyFont, $muted, 219, 414)

        $graphics.FillRectangle($dark, 506, 96, 1048, 166)
        $sceneItems = @(
            [pscustomobject]@{ Size = 48; X = 560; Label = "任务栏 48 px" },
            [pscustomobject]@{ Size = 24; X = 888; Label = "托盘 24 px" },
            [pscustomobject]@{ Size = 32; X = 1218; Label = "窗口图标 32 px" }
        )
        foreach ($item in $sceneItems) {
            $icon = [Drawing.Bitmap]::FromFile((Join-Path $WindowsPngPath ("{0}x{0}.png" -f $item.Size)))
            try { $graphics.DrawImage($icon, $item.X, 141, $item.Size, $item.Size) } finally { $icon.Dispose() }
            $graphics.DrawString($item.Label, $bodyFont, $white, $item.X + 70, 148)
        }

        $graphics.FillRectangle([Drawing.Brushes]::White, 506, 286, 1048, 160)
        $favicon = [Drawing.Bitmap]::FromFile((Join-Path $WindowsPngPath "32x32.png"))
        try { $graphics.DrawImage($favicon, 558, 331, 32, 32) } finally { $favicon.Dispose() }
        $graphics.DrawString("云智企办", $headingFont, $dark, 610, 324)
        $graphics.DrawString("浏览器标签 / WebView favicon · 32 px", $bodyFont, $muted, 610, 366)

        $graphics.FillRectangle($dark, 46, 474, 740, 374)
        $graphics.DrawString("深色界面", $headingFont, $white, 76, 502)
        $startup = [Drawing.Bitmap]::FromFile((Join-Path $WindowsPngPath "64x64.png"))
        try { $graphics.DrawImage($startup, 92, 584, 64, 64) } finally { $startup.Dispose() }
        $graphics.DrawString("启动页 / 首次引导 · 64 px", $bodyFont, $white, 184, 601)
        $darkWordmark = [Drawing.Bitmap]::FromFile($OnDarkWordmark)
        try { $graphics.DrawImage($darkWordmark, 112, 686, 422, 135) } finally { $darkWordmark.Dispose() }

        $graphics.FillRectangle([Drawing.Brushes]::White, 814, 474, 740, 374)
        $graphics.DrawString("浅色界面", $headingFont, $dark, 844, 502)
        $lightWordmark = [Drawing.Bitmap]::FromFile($OnLightWordmark)
        try { $graphics.DrawImage($lightWordmark, 908, 600, 422, 135) } finally { $lightWordmark.Dispose() }
        $graphics.DrawString("带名字的品牌图 · 云智企办", $bodyFont, $muted, 954, 760)
        Save-Png $canvas $OutputPath
    }
    finally {
        $dark.Dispose()
        $white.Dispose()
        $muted.Dispose()
        $titleFont.Dispose()
        $headingFont.Dispose()
        $bodyFont.Dispose()
        $graphics.Dispose()
        $canvas.Dispose()
    }
}

try {
    $masterDir = Join-Path $packPath "00-master"
    $windowsDir = Join-Path $packPath "01-windows"
    $windowsPngDir = Join-Path $windowsDir "png"
    $macDir = Join-Path $packPath "02-macos"
    $macIconsetDir = Join-Path $macDir "icon.iconset"
    $linuxDir = Join-Path $packPath "03-linux"
    $inAppDir = Join-Path $packPath "04-in-app"
    $previewDir = Join-Path $packPath "05-previews"
    $managedDirectories = if ($WindowsOnly) {
        @($masterDir, $windowsDir)
    }
    else {
        @($masterDir, $windowsDir, $macDir, $linuxDir, $inAppDir, $previewDir)
    }
    foreach ($directory in $managedDirectories) {
        Reset-ManagedDirectory $directory
    }
    $requiredDirectories = if ($WindowsOnly) {
        @($windowsPngDir)
    }
    else {
        @($windowsPngDir, $macIconsetDir)
    }
    foreach ($directory in $requiredDirectories) {
        Ensure-Directory $directory
    }

$source1024 = Join-Path $masterDir "appicon-source-1024.png"
$master1024 = Join-Path $masterDir "appicon-1024.png"
Save-Scaled $sourcePath 1024 $source1024
if ($SourceIsOptical) {
    Copy-Item -LiteralPath $source1024 -Destination $master1024 -Force
    Copy-Item -LiteralPath $sourcePath -Destination (Join-Path $masterDir "appicon-optical-source-1024.png") -Force
}
else {
    Save-OpticalFrame $source1024 1024 $master1024
    Copy-Item -LiteralPath $sourcePath -Destination (Join-Path $masterDir "图标方案1_珍珠白_原始1254.png") -Force
}
if (-not $WindowsOnly) {
    Copy-Item -LiteralPath $master1024 -Destination (Join-Path $repoRoot "desktop/build/appicon.png") -Force
}

function Save-FinalFrame([int]$Size, [string]$OutputPath) {
    if ($SourceIsOptical) {
        Save-Scaled $master1024 $Size $OutputPath
    }
    else {
        Save-OpticalFrame $source1024 $Size $OutputPath
    }
}

$windowsSizes = @(16, 20, 24, 30, 32, 36, 40, 48, 60, 64, 72, 96, 128, 256)
$windowsTaskbarMaster = Join-Path $windowsDir "taskbar-master-1024.png"
New-WindowsTaskbarMaster $master1024 $windowsTaskbarMaster
$icoFrames = foreach ($size in $windowsSizes) {
    $path = Join-Path $windowsPngDir ("{0}x{0}.png" -f $size)
    if ($size -le 64) {
        Save-Scaled $windowsTaskbarMaster $size $path
    }
    else {
        Save-FinalFrame $size $path
    }
    [pscustomobject]@{ Size = $size; Path = $path }
}
$packIco = Join-Path $windowsDir "icon.ico"
Write-Ico $packIco $icoFrames
Copy-Item -LiteralPath $packIco -Destination (Join-Path $repoRoot "desktop/build/windows/icon.ico") -Force
if ($WindowsOnly) {
    Write-Output "Generated Windows icon assets in $repoRoot and $packPath"
    return
}

$macMappings = @(
    [pscustomobject]@{ Name = "icon_16x16.png"; Size = 16 },
    [pscustomobject]@{ Name = "icon_16x16@2x.png"; Size = 32 },
    [pscustomobject]@{ Name = "icon_32x32.png"; Size = 32 },
    [pscustomobject]@{ Name = "icon_32x32@2x.png"; Size = 64 },
    [pscustomobject]@{ Name = "icon_128x128.png"; Size = 128 },
    [pscustomobject]@{ Name = "icon_128x128@2x.png"; Size = 256 },
    [pscustomobject]@{ Name = "icon_256x256.png"; Size = 256 },
    [pscustomobject]@{ Name = "icon_256x256@2x.png"; Size = 512 },
    [pscustomobject]@{ Name = "icon_512x512.png"; Size = 512 },
    [pscustomobject]@{ Name = "icon_512x512@2x.png"; Size = 1024 }
)
foreach ($mapping in $macMappings) {
    $path = Join-Path $macIconsetDir $mapping.Name
    Save-FinalFrame $mapping.Size $path
}
$icnsEntries = @(
    [pscustomobject]@{ Type = "icp4"; Path = (Join-Path $macIconsetDir "icon_16x16.png") },
    [pscustomobject]@{ Type = "icp5"; Path = (Join-Path $macIconsetDir "icon_32x32.png") },
    [pscustomobject]@{ Type = "icp6"; Path = (Join-Path $macIconsetDir "icon_32x32@2x.png") },
    [pscustomobject]@{ Type = "ic07"; Path = (Join-Path $macIconsetDir "icon_128x128.png") },
    [pscustomobject]@{ Type = "ic08"; Path = (Join-Path $macIconsetDir "icon_256x256.png") },
    [pscustomobject]@{ Type = "ic09"; Path = (Join-Path $macIconsetDir "icon_512x512.png") },
    [pscustomobject]@{ Type = "ic10"; Path = (Join-Path $macIconsetDir "icon_512x512@2x.png") },
    [pscustomobject]@{ Type = "ic11"; Path = (Join-Path $macIconsetDir "icon_16x16@2x.png") },
    [pscustomobject]@{ Type = "ic12"; Path = (Join-Path $macIconsetDir "icon_32x32@2x.png") },
    [pscustomobject]@{ Type = "ic13"; Path = (Join-Path $macIconsetDir "icon_128x128@2x.png") },
    [pscustomobject]@{ Type = "ic14"; Path = (Join-Path $macIconsetDir "icon_256x256@2x.png") }
)
$packIcns = Join-Path $macDir "icon.icns"
Write-Icns $packIcns $icnsEntries
Copy-Item -LiteralPath $packIcns -Destination (Join-Path $repoRoot "desktop/build/darwin/icon.icns") -Force

$linuxSizes = @(16, 24, 32, 48, 64, 128, 256, 512)
foreach ($size in $linuxSizes) {
    $repoLinuxPath = Join-Path $repoRoot ("desktop/build/linux/icons/hicolor/{0}x{0}/apps/kwu-desktop.png" -f $size)
    $packLinuxPath = Join-Path $linuxDir ("hicolor/{0}x{0}/apps/kwu-desktop.png" -f $size)
    Ensure-Directory ([IO.Path]::GetDirectoryName($repoLinuxPath))
    Ensure-Directory ([IO.Path]::GetDirectoryName($packLinuxPath))
    Save-FinalFrame $size $repoLinuxPath
    Copy-Item -LiteralPath $repoLinuxPath -Destination $packLinuxPath -Force
}
$repoSvg = Join-Path $repoRoot "desktop/build/appicon.svg"
$repoLinuxSvg = Join-Path $repoRoot "desktop/build/linux/icons/hicolor/scalable/apps/kwu-desktop.svg"
Write-SvgWrapper $master1024 $repoSvg
Write-SvgWrapper $master1024 $repoLinuxSvg
$packSvg = Join-Path $masterDir "appicon.svg"
Copy-Item -LiteralPath $repoSvg -Destination $packSvg -Force
$packLinuxSvg = Join-Path $linuxDir "hicolor/scalable/apps/kwu-desktop.svg"
Ensure-Directory ([IO.Path]::GetDirectoryName($packLinuxSvg))
Copy-Item -LiteralPath $repoLinuxSvg -Destination $packLinuxSvg -Force
$packPixmaps = Join-Path $linuxDir "pixmaps/kwu-desktop.png"
Ensure-Directory ([IO.Path]::GetDirectoryName($packPixmaps))
Copy-Item -LiteralPath $master1024 -Destination $packPixmaps -Force

$symbol = Join-Path $windowsPngDir "256x256.png"
$repoSymbol = Join-Path $repoRoot "desktop/frontend/src/assets/brand-symbol.png"
$repoFavicon = Join-Path $repoRoot "desktop/frontend/public/favicon.png"
Copy-Item -LiteralPath $symbol -Destination $repoSymbol -Force
Copy-Item -LiteralPath $symbol -Destination $repoFavicon -Force
Copy-Item -LiteralPath $symbol -Destination (Join-Path $inAppDir "brand-symbol.png") -Force
Copy-Item -LiteralPath $symbol -Destination (Join-Path $inAppDir "favicon.png") -Force
foreach ($size in @(16, 32, 48)) {
    $repoFaviconFrame = Join-Path $repoRoot ("desktop/frontend/public/favicon-{0}.png" -f $size)
    Save-FinalFrame $size $repoFaviconFrame
    Copy-Item -LiteralPath $repoFaviconFrame -Destination (Join-Path $inAppDir ("favicon-{0}.png" -f $size)) -Force
}

$wordmarkSource = Join-Path $masterDir "brand-wordmark-source.png"
if (-not (Test-Path -LiteralPath $wordmarkSource)) {
    $repoDarkWordmark = Join-Path $repoRoot "desktop/frontend/src/assets/brand-wordmark-on-dark.png"
    $fallbackWordmark = if (Test-Path -LiteralPath $repoDarkWordmark) {
        $repoDarkWordmark
    }
    else {
        Join-Path $repoRoot "desktop/frontend/src/assets/brand-wordmark.png"
    }
    Copy-Item -LiteralPath $fallbackWordmark -Destination $wordmarkSource
}
$repoWordmark = Join-Path $repoRoot "desktop/frontend/src/assets/brand-wordmark.png"
$repoWordmarkOnDark = Join-Path $repoRoot "desktop/frontend/src/assets/brand-wordmark-on-dark.png"
New-Wordmarks $wordmarkSource $symbol $repoWordmark $repoWordmarkOnDark
Copy-Item -LiteralPath $repoWordmark -Destination (Join-Path $inAppDir "brand-wordmark.png") -Force
Copy-Item -LiteralPath $repoWordmarkOnDark -Destination (Join-Path $inAppDir "brand-wordmark-on-dark.png") -Force

New-FullSizePreview (Join-Path $previewDir "全尺寸效果预览.png") $repoWordmark $windowsPngDir $windowsSizes
New-ScenePreview (Join-Path $previewDir "不同场景效果预览.png") $windowsPngDir $repoWordmark $repoWordmarkOnDark

    Write-Output "Generated icon assets in $repoRoot and $packPath"
}
finally {
    if ($temporarySourcePath -and (Test-Path -LiteralPath $temporarySourcePath)) {
        Remove-Item -LiteralPath $temporarySourcePath -Force
    }
}
