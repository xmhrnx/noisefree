// A tiny LADSPA plugin used by the host tests: halves (or scales) its input.
#include <stdlib.h>
#include "../host.h"

typedef struct { LADSPA_Data *in, *out, *gain; } Gain;

static LADSPA_Handle inst(const LADSPA_Descriptor *d, unsigned long r) { (void)d; (void)r; return calloc(1, sizeof(Gain)); }
static void conn(LADSPA_Handle h, unsigned long p, LADSPA_Data *b) {
	Gain *g = h;
	if (p == 0) g->in = b; else if (p == 1) g->out = b; else g->gain = b;
}
static void run(LADSPA_Handle h, unsigned long n) {
	Gain *g = h;
	for (unsigned long i = 0; i < n; i++) g->out[i] = g->in[i] * *g->gain;
}
static void cleanup(LADSPA_Handle h) { free(h); }

static const int descs[] = {0x1 | 0x8, 0x2 | 0x8, 0x1 | 0x4};
static const char *const names[] = {"Input", "Output", "Gain"};
static const LADSPA_PortRangeHint hints[] = {{0, 0, 0}, {0, 0, 0}, {0x1 | 0x2 | 0xC0, 0, 1}};
static const LADSPA_Descriptor desc = {
	42, "test_gain", 0, "Test Gain", "noisefree", "none", 3, descs, names, hints, NULL,
	inst, conn, NULL, run, NULL, NULL, NULL, cleanup,
};
const LADSPA_Descriptor *ladspa_descriptor(unsigned long i) { return i == 0 ? &desc : NULL; }
