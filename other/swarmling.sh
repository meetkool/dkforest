#!/bin/bash

# resources:
# https://stackoverflow.com/questions/8789729/how-to-zero-pad-a-sequence-of-integers-in-bash-so-that-all-have-the-same-width
# https://tldp.org/LDP/Bash-Beginners-Guide/html/sect_07_01.html
# https://linuxize.com/post/bash-for-loop/
# https://unix.stackexchange.com/questions/107048/pass-multiple-commands-to-flock
# https://superuser.com/questions/515313/force-wget-to-timeout#515315

# usage syntax
help(){
	echo "Download a large file quickly through Tor with concurrent circuits."
	echo "Usage: (argument positions are hardcoded.)"
	echo "swarmling.sh -t <concurrent_circuits> -n <largest_fragment> -u <url_without_part> -o <filename>"
	echo "Note:"
	echo "Number of fragments is last number on server +1 as index starts from 0."
}

# function to be forked in background to download
function dlThread(){
	# enter tempdir
	cd .swarmling_buffer

	# do while loop
	success=1
	while [ ${success} = 1 ]; do
		# download through a different circuit each time, loop on timeout
		# uses CURL to kill off a tor circuit if it is under 50KB/s for 5 seconds (hardcoded for now)
		#timeout --preserve-status -s KILL 120 torsocks -i wget --quiet -c --tries=1 --content-disposition "$1"
		curl -Y 50000 -y 5 -O -C - -s -J -U "${RANDOM}":"${RANDOM}" -x socks5h://localhost:9050 "$1"
		if [ $? = 0 ]; then
			success=0
		fi
	done

	# update progress
	flock -x ./progress bash -c 'echo "$(($(cat progress)+1))" > progress';
	flock -x ./threads bash -c 'echo "$(($(cat threads)-1))" > threads';
}

function testThread(){
	cd .swarmling_buffer/

	sleep $((${RANDOM}%5))
	echo "Thread done."

	# update progress
	flock -x ./progress bash -c 'echo "$(($(cat progress)+1))" > progress';
}

function testThreadConcurrent(){
	cd .swarmling_buffer/

	sleep $((${RANDOM}%5))
	echo "Concurrent thread done."

	# update progress and decrease thread counter
	flock -x ./progress bash -c 'echo "$(($(cat progress)+1))" > progress';
	flock -x ./threads bash -c 'echo "$(($(cat threads)-1))" > threads';
}

if [ "$1" = "" -o "$1" = "-h" -o "$1" = "--help" ]; then
	help;
	exit;
else
	threadsMax="$2"
	largest_fragment="$4"
	url="$6"
	output="$8"

	# clean and make temp download dir
	if [ -a .swarmling_buffer ]; then
		rm -r .swarmling_buffer
	fi
	mkdir .swarmling_buffer
	echo 0 > .swarmling_buffer/progress
	echo 0 > .swarmling_buffer/threads

	# ---- v

	# keep running concurrent threads up to ${threadsMax} while until there are no more fragments to download
	running=1
	i=0
	while [ ${running} = 1 ]; do
		# run concurrent thread in background if there are less than specified threads running
		if [ $(flock -x .swarmling_buffer/threads cat .swarmling_buffer/threads) -lt ${threadsMax} ]; then
			# add one thread to thread counter
			flock -x .swarmling_buffer/threads bash -c 'echo "$(($(cat .swarmling_buffer/threads)+1))" > .swarmling_buffer/threads';

			# run thread in background
			dlThread "${url}.part$(printf '%05d' $i)" &
			i=$(($i+1))
			#testThreadConcurrent &
		else
			#debug
			sleep 1
		fi

		# exit loop if downloaded + in progress are equal to ${largest_thread}
		if [ $(flock -x .swarmling_buffer/progress cat .swarmling_buffer/progress) -ge $(( ${largest_fragment} - $(flock -x .swarmling_buffer/threads cat .swarmling_buffer/threads) )) ]; then
			running=0;
		fi
		echo $(cat .swarmling_buffer/progress)/${largest_fragment}
	done

	# wait for all threads to be done
	while [ $(flock -x .swarmling_buffer/threads cat .swarmling_buffer/threads) -gt 0 ]; do
		echo $(cat .swarmling_buffer/progress)/${largest_fragment}
		sleep 1
	done


	# ---- ^

	# indicate completion
	echo "DONE"

	# merge downloaded files
	cat .swarmling_buffer/*part* > "${output}"

	#remove buffer dir
	rm -r .swarmling_buffer/
fi
