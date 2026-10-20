# Trust agents

A trust agent runs on an element and answers Jane's requests for evidence. An element lists its trust agents as **endpoints**: a name, a URL and the [protocol](protocols.md) to use, e.g.

```yaml
endpoints:
  tarzan:
      endpoint: http://192.0.2.10:8530
      protocol: A10HTTPRESTv2
```

An intent is attested against one endpoint of an element; the attest page and jp choose which.

## tarzan

tarzan is Jane's reference trust agent, in `tarzan/` and the tarzan package. It offers APIs selected by flags (`-sys`, `-tpm2`, `-uefi`, `-ima`, `-txt`), listens on port 8530, and runs as root because it needs the TPM and the logs under `/sys/kernel/security`. It does the TPM work natively through go-tpm: PCR reads, quotes, TPM2_GetTime, capabilities and credential activation. Since 1.5.3 it returns the raw signed attestation with every quote and gettime, so Jane can verify the signature.

tarzan speaks plain HTTP and has no authentication (#58): firewall its port so only Jane can reach it. It only reads logs from their standard locations unless started with `-unsafe`, which must never be used. Details: [Running](running.md#tarzan), [Hardening](hardening.md#10-tarzan).

jp, installed with tarzan, registers the element with Jane and records its endpoints from the provisioning file.

## ratsd

[ratsd](https://github.com/veraison/ratsd) is Veraison's attestation daemon: it gathers evidence from leaf attesters (e.g. TSM reports for confidential VMs, NVIDIA GPUs) into one CMW collection. Jane talks to it with the `RATSD` protocol, by default on port 8895. To use it, install ratsd on the element, add its endpoint (commented out in the provisioning example) and use the `std::intent::ratsd::*` standard intents, with the `chares_*` rules.

## Writing another trust agent

Any program that answers the HTTP API of `A10HTTPRESTv2` can stand in for tarzan; tarzan's source (`tarzan/tpm2`, `tarzan/uefi`, ...) is the reference for the request parameters and response bodies, which are listed in [Functions](functions.md).
