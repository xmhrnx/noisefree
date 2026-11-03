// Minimal LADSPA 1.1 ABI declarations and a portable dynamic loader.
#ifndef NOISEFREE_LADSPA_HOST_H
#define NOISEFREE_LADSPA_HOST_H

typedef float LADSPA_Data;
typedef void *LADSPA_Handle;

typedef struct {
	int HintDescriptor;
	LADSPA_Data LowerBound;
	LADSPA_Data UpperBound;
} LADSPA_PortRangeHint;

typedef struct _LADSPA_Descriptor {
	unsigned long UniqueID;
	const char *Label;
	int Properties;
	const char *Name;
	const char *Maker;
	const char *Copyright;
	unsigned long PortCount;
	const int *PortDescriptors;
	const char *const *PortNames;
	const LADSPA_PortRangeHint *PortRangeHints;
	void *ImplementationData;
	LADSPA_Handle (*instantiate)(const struct _LADSPA_Descriptor *, unsigned long);
	void (*connect_port)(LADSPA_Handle, unsigned long, LADSPA_Data *);
	void (*activate)(LADSPA_Handle);
	void (*run)(LADSPA_Handle, unsigned long);
	void (*run_adding)(LADSPA_Handle, unsigned long);
	void (*set_run_adding_gain)(LADSPA_Handle, LADSPA_Data);
	void (*deactivate)(LADSPA_Handle);
	void (*cleanup)(LADSPA_Handle);
} LADSPA_Descriptor;

typedef const LADSPA_Descriptor *(*LADSPA_Descriptor_Function)(unsigned long);

void *noisefree_open(const char *path, char *err, int errlen);
void noisefree_close(void *lib);
const LADSPA_Descriptor *noisefree_descriptor(void *lib, unsigned long index);

LADSPA_Handle noisefree_instantiate(const LADSPA_Descriptor *d, unsigned long rate);
void noisefree_connect(const LADSPA_Descriptor *d, LADSPA_Handle h, unsigned long port, LADSPA_Data *buf);
void noisefree_activate(const LADSPA_Descriptor *d, LADSPA_Handle h);
void noisefree_run(const LADSPA_Descriptor *d, LADSPA_Handle h, unsigned long n);
void noisefree_cleanup(const LADSPA_Descriptor *d, LADSPA_Handle h);

#endif
