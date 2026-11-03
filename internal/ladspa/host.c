#include <stdio.h>
#include <string.h>
#include "host.h"

#ifdef _WIN32
#include <windows.h>
void *noisefree_open(const char *path, char *err, int errlen) {
	HMODULE m = LoadLibraryA(path);
	if (!m) snprintf(err, errlen, "LoadLibrary failed (error %lu)", GetLastError());
	return (void *)m;
}
void noisefree_close(void *lib) { FreeLibrary((HMODULE)lib); }
static void *noisefree_sym(void *lib, const char *name) { return (void *)GetProcAddress((HMODULE)lib, name); }
#else
#include <dlfcn.h>
void *noisefree_open(const char *path, char *err, int errlen) {
	void *m = dlopen(path, RTLD_NOW | RTLD_LOCAL);
	if (!m) snprintf(err, errlen, "%s", dlerror());
	return m;
}
void noisefree_close(void *lib) { dlclose(lib); }
static void *noisefree_sym(void *lib, const char *name) { return dlsym(lib, name); }
#endif

const LADSPA_Descriptor *noisefree_descriptor(void *lib, unsigned long index) {
	LADSPA_Descriptor_Function fn = (LADSPA_Descriptor_Function)noisefree_sym(lib, "ladspa_descriptor");
	return fn ? fn(index) : NULL;
}

LADSPA_Handle noisefree_instantiate(const LADSPA_Descriptor *d, unsigned long rate) { return d->instantiate(d, rate); }
void noisefree_connect(const LADSPA_Descriptor *d, LADSPA_Handle h, unsigned long port, LADSPA_Data *buf) { d->connect_port(h, port, buf); }
void noisefree_activate(const LADSPA_Descriptor *d, LADSPA_Handle h) { if (d->activate) d->activate(h); }
void noisefree_run(const LADSPA_Descriptor *d, LADSPA_Handle h, unsigned long n) { d->run(h, n); }
void noisefree_cleanup(const LADSPA_Descriptor *d, LADSPA_Handle h) {
	if (d->deactivate) d->deactivate(h);
	if (d->cleanup) d->cleanup(h);
}
