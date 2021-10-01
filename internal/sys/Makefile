sys_generated.go: sys_defs.go
	go tool cgo -godefs sys_defs.go > sys_generated.go
	#rm -rf _obj/
	go fmt sys_generated.go

clean:
	rm sys_generated.go
