# Coarse-grained educational model — assumptions and equations

The default live simulator deliberately models a **small number of coupled
signals**, not the biochemical network of E. coli. It is provided to show the
end-to-end data flow and responsive visual controls. Do not cite its output as
experimentally validated or as vEcoli results.

## State and units

- `time_min`: minutes of *simulated* time; one web tick advances 2 min.
- `glucose_mM`: illustrative bulk substrate concentration (millimolar).
- `atp_mM`: illustrative intracellular ATP concentration (millimolar).
- `biomass_fg`: mass of one reference cell (femtograms), initial 280 fg.
- `growth_per_hour`: current relative specific growth rate in h⁻¹.
- `translation_activity`: dimensionless 0–1 activity proxy, not protein count.
- `energy_index`: `clamp(ATP / 3 mM, 0, 1)`, **not** biochemical adenylate energy charge.
- `division_progress`: fraction toward doubled reference biomass, illustrative.
- `medium`: rich, limited or starved (controls external glucose replenishment).

## Update rule (2-minute Euler ticks)

Let `g` be glucose in mM; `a` be ATP in mM; `b` be biomass in fg.

```text
n = g / (0.65 + g)                               # substrate saturation
energy_index = clamp(a / 3, 0, 1)
translation = (0.08 + 0.92*n) * (0.18 + 0.82*energy_index)
mu = 0.93 * n * translation                      # per hour
uptake = 0.028 * g/(0.35 + g) * clamp(b/280, 0.5, 2)
feed = { rich: 0.060, limited: 0.006, starved: 0.0 }
g_next = clamp(g + dt*[feed*(1-g/5)-uptake], 0, 7)
a_next = clamp(a + dt*(0.024*n - 0.016 - 0.010*translation), 0.15, 3.5)
b_next = b * exp(mu * dt / 60)
if b_next >= 560: emit division; b_next /= 2
```

Saturation constants and rates were **chosen for clear demonstration**, not
fitted to empirical measurements. Equations are update-order-dependent; see
`internal/sim/model.go` as the authoritative implementation.

## Biology-to-visualization mappings

| Observable | Rendering behavior | Not claimed |
|---|---|---|
| Biomass | Scales cell length ~cube-root relative to initial value | Exact cell volume or daughter topology |
| Translation activity | Changes emission/pulse of representative ribosomes | Accurate stochastic translation kinetics |
| ATP index | Changes appearance of ATP synthase markers | ATP synthase catalytic state |
| Time | Subtle flagellar and chromosome illustration animation | Real molecular motion trajectories |
| Medium | Controls glucose replenishment in the coarse model | Experimental minimal-media response |

An actual scientific model needs calibrated uptake/maintenance rates, osmotic
and electrochemical constraints, gene expression kinetics, polymer geometry,
measured abundances, biologically meaningful division rules, parameter
uncertainty, and independent validation datasets.
