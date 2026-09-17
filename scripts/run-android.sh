#!/bin/sh

set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
android_sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}
java_home_path=${JAVA_HOME:-}

if [ -z "$android_sdk" ]; then
	user_home_dir=${HOME:-}
	for candidate in \
		"$project_root/.android-sdk" \
		"$user_home_dir/Library/Android/sdk" \
		"$user_home_dir/Android/Sdk" \
		"/opt/homebrew/share/android-commandlinetools" \
		"/usr/local/share/android-sdk"
	do
		if [ -x "$candidate/platform-tools/adb" ]; then
			android_sdk=$candidate
			break
		fi
	done
fi

if [ -z "$java_home_path" ]; then
	for candidate in \
		"/Applications/Android Studio.app/Contents/jbr/Contents/Home" \
		"/opt/homebrew/opt/openjdk@17" \
		"/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home" \
		"/usr/local/opt/openjdk@17" \
		"/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"
	do
		if [ -x "$candidate/bin/java" ]; then
			java_home_path=$candidate
			break
		fi
	done
fi

if [ -z "$android_sdk" ] || [ ! -x "$android_sdk/platform-tools/adb" ]; then
	echo "Android SDK/adb not found. Set ANDROID_HOME or ANDROID_SDK_ROOT." >&2
	exit 1
fi
if [ -z "$java_home_path" ] || [ ! -x "$java_home_path/bin/java" ]; then
	echo "JDK 17 not found. Set JAVA_HOME." >&2
	exit 1
fi
if [ ! -x "$project_root/android/gradlew" ]; then
	echo "Gradle wrapper not found under android/." >&2
	exit 1
fi

export ANDROID_HOME="$android_sdk"
export ANDROID_SDK_ROOT="$android_sdk"
export JAVA_HOME="$java_home_path"
export PATH="$java_home_path/bin:$android_sdk/platform-tools:$PATH"

mkdir -p "$project_root/android/app/libs"

echo "Generating the Go/Ebitengine library for Android arm64"
cd "$project_root"
go run github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.9.11 \
	bind \
	-target android/arm64 \
	-androidapi 23 \
	-javapkg com.olivierh59500.tcbscroller \
	-o android/app/libs/tcbscroller.aar \
	./mobile

echo "Building the debug APK"
"$project_root/android/gradlew" \
	-p "$project_root/android" \
	--console=plain \
	assembleDebug

adb_path="$android_sdk/platform-tools/adb"
apk_path="$project_root/android/app/build/outputs/apk/debug/app-debug.apk"
device_count=$("$adb_path" devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')

if [ "$device_count" -ne 1 ]; then
	echo "Exactly one authorized Android device is required (found: $device_count)." >&2
	echo "Unlock the Pixel, enable USB debugging, and accept its RSA prompt." >&2
	"$adb_path" devices -l >&2
	exit 1
fi

echo "Installing the APK"
"$adb_path" install -r "$apk_path"

echo "Launching TCB Multi-Plane 3D Scroller"
"$adb_path" shell am force-stop com.olivierh59500.tcbscroller
"$adb_path" shell am start -n com.olivierh59500.tcbscroller/.MainActivity
