# Getting Started

In the following sections we explain how to get started with Jane and explore some of its features. We assume that you have Jane and Tazan up and [running](running.md) and that you can get to the web UI. 

NB: It is likely that Tarzan will need to run as root.

   * Overview of the UI
   * Loading the standard intents   
   * Creating an element
   * Your first attestation
   * Creating and expected value
   * PCRs
   * Opaque Objects
   * Quotes

# Overview of the UI

Point your browser at the UI, eg: (http://127.0.0.1:8540) if you are running on a local machine on the default port. You should see something like this:

![Jane WebUI Home Page](images/janewebuihomepage.png "Jane WebUI Home Page")

This shows the overall status of the system. The upper part showing the contents of the database and the lower part the congfiguration of the system. The lower half shows the configuration: how janeserver was started and with what paratmers, what services are running and on what ports, the state of the MQTT messagebus, Mongo databaes and the logging file.

The top bar is available on all pages and clicking on the home icon on the left-hand side will always bring you back to this page.

## Help and About

If you need help or wish for more history of Jane, then use the Help menu which has these Help... and About... options

# Loading the standard intents

The first thing we need to do is to load the `standard intents` which are the attestation instructions. This is done simply by going to the top menu, then New... and Load/Reload Standard Intents.  These are retrieved from github and are very, very occasionally updated. It doesn't matter how many times you download these, but once they are downloaded they will be kept in Jane's database.

![Load/Reload Standard Intents](images/loadstandardintents.png "Load/Reload Standard Intents")

If all is good then you'll get a page like this:

![Standard Intents](images/standardintents.png "Standard Intents")

# Creating an Element
For this I will assume that you have Jane and Tazan on the same local machine and you have a TPM 2.0 ready for use, and that /dev/tpm0 and /dev/tpmrm0 exist! 

## TPM Provisioning
You will need to generate the EK and AK keys and store them in suitable handles. See  https://github.com/iolivergithub/TPMCourse/blob/master/docs/keys.md#special-keys how to do this.

Jane for historical reasons defaults to 0x810100EE and 0x810100AA for the two keys (you can figure out which is which). You are free to use any other handles you want as long as the TPM allows you. For example, this means I have the two keys:

```bash
$ tpm2_getcap handles-persistent
- 0x810100AA
- 0x810100EE
```

If you can generate a quote using `tpm2_quote` then you're probably good. Check you can get `tpm2_pcrread` working too. It is also a good idea to check if you have a UEFI eventlog as well. 

## New Element
Now we need to create a new element:  Top menu -> New... -> Element which gives a page like this:

![New Element Template](images/newelemeenttemplate.png "New Element Template")

Now you have to be VERY careful as Jane makes absolutely no attempt to do any error correction. You can try pretty printing which serves as some form, but, well, good luck. If it works and you need to make changes then you will need to access the mongo database (Mongo Compass is a good UI).

Pretty much everything in the tempate is default, but fill in those with asterisks. For example:

```json
 "name": "Test VM",
 "description": "This is a test VM",
```

The IP addresses of the endpoins doesn't need to be changed if we are working locally. You only need to care bout the tarzan endpoint - we are not using RATSD yet (one day maybe)

Tags are just useful labels, for example I tend to use them like this: (just make sure you get the commas and quotes correct)
```json
    "tags":
    [
        "linux",
        "x86",
        "vm"
    ],
```

Then the host section. Firstly run these commands in a teminal (NB: if you don't have a machine-id file, just leave it blank)

```bash
$ hostname
debianwork
$ uname -a
Linux debianwork 6.12.57+deb13-amd64 #1 SMP PREEMPT_DYNAMIC Debian 6.12.57-1 (2025-11-05) x86_64 GNU/Linux
$ cat /etc/machine-id 
abcdefg123456786
```

Then write into the host section, something like this:

```json
    "host":
    {
        "os": "linux",
        "arch": "amd64",
        "hostname": "debianwork",
        "machineid": "abcdefg123456786"
    },
```

If you changed the TPM EK and AK handles, then change these in the file as appropriate. Otherwise leave everything else as it is.

Press SUBMIT...

If it works....then you get something like below, if not, use a database editor to delete it and try again.

![A New Element - Success!!](images/listofelements.png "A New Element - Success!!")

![The New Element In Detail](images/element.png "The New Element In Detail")

# Your First Attestation
Go to the attest page (via top menu) and you will see something like this:

![Attest](images/attest.png "Attest")

Select your element and endpoint, eg: DebianWorkVM - tarzan, then select an intent, in this case we will use `System Information: sys info` and then ensure the "Attest Only" button is selected. Click SUBMIT, and if Jane has successfully talked with Tarzan you'll be taken a page detailing the claim:

![A Claim](images/aclaim.png "A Claim")

Congratulations!

Now we will try running a rule, and the most important one is that tarzan is running in its safe mode (in its unsafe mode you effectively expose your entier file system to the internet as root....#WCGW). Go back to the attest page and select your element again, the system information intent and now the rule named "sys_taRunningSafely".  This time ensure "Attest and Verify" is selected and click on Submit. This time if all goes well you'll get to a result page:

![A Result](images/aresult.png "A Result")

# Creating an Expected Value
In this example we will create an "expected value" for the machine ID. Make a note of the machine ID that you included with your element. If you do not have a machine ID, then use your imagination.

Go to Top Menu->New->Expected Value

![Creating an Expected Value](images/evpage.png "Creating an Expected Value")

Ignore ItemID, but fill in Name and Description, select which element you which to associate with this (same element as above), which Intent (System Information sys/info) and then click on the button SYS/MachineID which gives you the template for this. Change the asteriks to the machine ID you are using:

![Creating an Expected Value - Example](images/evexample.png "Creating an Expected Value - Example")

And click Submit, and if all goes well you'll see a list of expected values.

![Expected Values](images/evs.png "Expected Values")

Go back to the attest page, and select the element, the intent (System Information) and now the THREE rules for system information checks:

![Attesting and Verifying MachineID](images/midattest.png "Attesting and Verifying MachineID")

Submit and you should get a result page like this:

![Attesting and Verifying MachineID - Result](images/midverify.png "Attesting and Verifying MachineID - Result")

# PCRs

To get PCRs, simply ask for them - there are no rules to run over these so select attest only.

NB: on some modern PCs, SHA1 is available, but not populated. This causes tarzan not to work (I will fix this bug one day https://github.com/iolivergithub/jane/issues/15) and you can't get the list. This is extremely annoying

![Attesting PCRs](images/pcrs.png "Attesting PCRs")

And the claim might look like this:

![Attesting PCRs - Claim](images/pcrresults.png "Attesting PCRs - Claim")

# Opaque Objects
A useful feature is to collect information about certain values, for example, PCRs. Take note of the SHA1 and SHA256 values for PCR0. In my case these are:  c4b0dc9f0f6185483ec8b40c3a8c06fdef7187e0  and  fcf4d7b42f073a5cf8fa5929cc2f5ac07039ca7a999b7e078faa2af6870a1250 respectively.

Go to Top Menu->New->Opaque Object and fill in the details like below (so the same for the SHA256 hash too)

![Example Opaque Object](images/oo.png "Example Opaque Object")

Now if you attest for PCRs again, Jane will recognise these values for convenience and if you hover over these with the pointer you will get summary information (you can click on them too).

![Example Opaque Object - PCR Recognition](images/oopcrs.png "Example Opaque Object - PCR Recognition")

# Quotes
To collect information for quotes it is necessary to decide which intents and thus which sets of information you want to collect. The easiest way of doing this is to start is to attest only for an intent, copy the information, and then generate an expected value as before. Here is an example procedure:

   1. Attest your element for the intent x86 UEFI CRTM only
   1. Take note of the PCR Digest and Firmware fields as below (my example: saDQFcKYWDAvgbUqJ+yuh7lwD7QrdcbMasF3/iXqKTw= and 2019102300163636)
   1. Create a new Expected Value, but use the TPM/QUote template, Fill in the asterisks with the above information. SUBMIT once. If you submit twice you will have to edit the database...volunteers to fix this are welcome!
   1. Go the attest page for your element, select the intent (x86 UEFI CRTM only) and now select the 6 tpm2 rules.
   1. Check your results!
   1. Repeat the above for other intents

![Quote - Claim](images/quote.png "Quote - Claim")

![EV Quote - Claim](images/evquote.png "EV Quote - Claim")

![Attest Quote](images/quoteattest.png "Attest Quote")

![Attest Quote Result](images/quoteresult.png "Attest Quote Result")
