```
[appadm@dgx-a100 goose]$ nvidia-smi -L
GPU 0: NVIDIA A100-SXM4-40GB (UUID: GPU-9630cc21-35cd-3bd5-42c3-ce34229ec929)
GPU 1: NVIDIA A100-SXM4-40GB (UUID: GPU-f175d98b-41b4-9b45-e6b4-881c9bf5b527)
MIG 1g.5gb      Device  0: (UUID: MIG-e243b789-88dd-5d74-bf9e-74a65024ef0d)
MIG 1g.5gb      Device  1: (UUID: MIG-f6c30480-1419-5316-91d7-b32c06559be2)
MIG 1g.5gb      Device  2: (UUID: MIG-f7a8fc4c-4d42-5ab8-9205-6fdd8f8e2a5c)
MIG 1g.5gb      Device  3: (UUID: MIG-6d7f4005-c974-55d9-b196-698f0141392c)
MIG 1g.5gb      Device  4: (UUID: MIG-c05435c4-539c-57db-9f1f-ec80705a7a56)
MIG 1g.5gb      Device  5: (UUID: MIG-bdd2a903-cb33-5f00-9ef5-b924d3af9874)
MIG 1g.5gb      Device  6: (UUID: MIG-1e12911b-1690-5373-9a61-7d5ab9ab2d13)
GPU 2: NVIDIA A100-SXM4-40GB (UUID: GPU-f3110e1c-d733-6856-5934-8c12178ab1b2)
MIG 1g.5gb      Device  0: (UUID: MIG-a738dbfb-ef98-5b4d-a6dc-dab3c0e74987)
MIG 1g.5gb      Device  1: (UUID: MIG-1951ae35-7d02-5d06-878a-283737bc6cb9)
MIG 1g.5gb      Device  2: (UUID: MIG-9bacde92-aeb3-52e9-b6b5-47429a199ccf)
MIG 1g.5gb      Device  3: (UUID: MIG-09480519-7314-5881-bb63-d1b36c07e3fb)
MIG 1g.5gb      Device  4: (UUID: MIG-ad0e24f2-591b-58f7-80b4-f46be4e5823e)
MIG 1g.5gb      Device  5: (UUID: MIG-ae761056-95ab-5c9d-bb5d-8eb1f083bc34)
MIG 1g.5gb      Device  6: (UUID: MIG-98c42978-a4b0-536b-902a-f5ec0d069a2c)
GPU 3: NVIDIA A100-SXM4-40GB (UUID: GPU-dd61a59b-8108-be0f-25cd-5a3f39c1b52d)
MIG 1g.5gb      Device  0: (UUID: MIG-a59963c2-8e8e-5a1f-a730-3dab07261ab4)
MIG 1g.5gb      Device  1: (UUID: MIG-81dc7abe-8875-5c91-bf9e-19a753bf19ec)
MIG 1g.5gb      Device  2: (UUID: MIG-957a0885-da7f-5917-830e-90d4c49f882c)
MIG 1g.5gb      Device  3: (UUID: MIG-8ce06a54-22c3-51a1-8523-3c0cc77526b1)
MIG 1g.5gb      Device  4: (UUID: MIG-962cdcb7-5822-52bb-ba3f-314b705899d6)
MIG 1g.5gb      Device  5: (UUID: MIG-3adb31ea-b4aa-5a61-9f4a-da124b1ae803)
MIG 1g.5gb      Device  6: (UUID: MIG-72718b9b-7637-5ef2-861b-97dec734fed9)
GPU 4: NVIDIA A100-SXM4-40GB (UUID: GPU-82998f26-1ada-823d-06e3-cc393c651b63)
MIG 7g.40gb     Device  0: (UUID: MIG-9c0c7e6c-03c6-5756-b62a-9bcdd7bde458)
GPU 5: NVIDIA A100-SXM4-40GB (UUID: GPU-d778b998-ccd9-a6ed-fe97-53db32fca5a5)
MIG 7g.40gb     Device  0: (UUID: MIG-ec779ec1-7f40-5729-ba56-23660ed49606)
GPU 6: NVIDIA A100-SXM4-40GB (UUID: GPU-6f1ea379-b2d8-2d21-c942-20bb3a3264cb)
MIG 7g.40gb     Device  0: (UUID: MIG-80a504d7-322d-5e3f-b1f5-86caabb7f136)
GPU 7: NVIDIA A100-SXM4-40GB (UUID: GPU-e6b26b84-1fea-086e-ee0a-025a50970c2a)
MIG 7g.40gb     Device  0: (UUID: MIG-ec7f2144-0018-5959-834b-7573acffdadc)
```

```
[appadm@dgx-a100 goose]$ nvidia-smi
Mon Jan 26 16:22:43 2026
+-----------------------------------------------------------------------------------------+
| NVIDIA-SMI 550.163.01             Driver Version: 550.163.01     CUDA Version: 12.4     |
|-----------------------------------------+------------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id          Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |           Memory-Usage | GPU-Util  Compute M. |
|                                         |                        |               MIG M. |
|=========================================+========================+======================|
|   0  NVIDIA A100-SXM4-40GB          Off |   00000000:07:00.0 Off |                    0 |
| N/A   31C    P0             55W /  400W |       1MiB /  40960MiB |      0%      Default |
|                                         |                        |             Disabled |
+-----------------------------------------+------------------------+----------------------+
|   1  NVIDIA A100-SXM4-40GB          Off |   00000000:0F:00.0 Off |                   On |
| N/A   38C    P0            128W /  400W |    4480MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   2  NVIDIA A100-SXM4-40GB          Off |   00000000:47:00.0 Off |                   On |
| N/A   35C    P0             96W /  400W |      88MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   3  NVIDIA A100-SXM4-40GB          Off |   00000000:4E:00.0 Off |                   On |
| N/A   36C    P0             96W /  400W |      88MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   4  NVIDIA A100-SXM4-40GB          Off |   00000000:87:00.0 Off |                   On |
| N/A   41C    P0            102W /  400W |       2MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   5  NVIDIA A100-SXM4-40GB          Off |   00000000:90:00.0 Off |                   On |
| N/A   42C    P0            103W /  400W |       2MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   6  NVIDIA A100-SXM4-40GB          Off |   00000000:B7:00.0 Off |                   On |
| N/A   41C    P0             98W /  400W |       2MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+
|   7  NVIDIA A100-SXM4-40GB          Off |   00000000:BD:00.0 Off |                   On |
| N/A   41C    P0             98W /  400W |       2MiB /  40960MiB |     N/A      Default |
|                                         |                        |              Enabled |
+-----------------------------------------+------------------------+----------------------+

+-----------------------------------------------------------------------------------------+
| MIG devices:                                                                            |
+------------------+----------------------------------+-----------+-----------------------+
| GPU  GI  CI  MIG |                     Memory-Usage |        Vol|      Shared           |
|      ID  ID  Dev |                       BAR1-Usage | SM     Unc| CE ENC DEC OFA JPG    |
|                  |                                  |        ECC|                       |
|==================+==================================+===========+=======================|
|  1    7   0   0  |            4404MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 2MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1    8   0   1  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1    9   0   2  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1   10   0   3  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1   11   0   4  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1   12   0   5  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  1   13   0   6  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2    7   0   0  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2    8   0   1  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2    9   0   2  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2   11   0   3  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2   12   0   4  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2   13   0   5  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  2   14   0   6  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3    7   0   0  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3    8   0   1  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3    9   0   2  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3   11   0   3  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3   12   0   4  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3   13   0   5  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  3   14   0   6  |              13MiB /  4864MiB    | 14      0 |  1   0    0    0    0 |
|                  |                 0MiB /  8191MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  4    0   0   0  |               2MiB / 40327MiB    | 98      0 |  7   0    5    1    1 |
|                  |                 1MiB / 65536MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  5    0   0   0  |               2MiB / 40327MiB    | 98      0 |  7   0    5    1    1 |
|                  |                 1MiB / 65536MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  6    0   0   0  |               2MiB / 40327MiB    | 98      0 |  7   0    5    1    1 |
|                  |                 1MiB / 65536MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+
|  7    0   0   0  |               2MiB / 40327MiB    | 98      0 |  7   0    5    1    1 |
|                  |                 1MiB / 65536MiB  |           |                       |
+------------------+----------------------------------+-----------+-----------------------+

+-----------------------------------------------------------------------------------------+
| Processes:                                                                              |
|  GPU   GI   CI        PID   Type   Process name                              GPU Memory |
|        ID   ID                                                               Usage      |
|=========================================================================================|
|    1    7    0     229163      C   ./gpu_burn                                   4384MiB |
+-----------------------------------------------------------------------------------------+
```


```dcgm-exporter 로그
ime=2026-01-26T07:24:57.638Z level=DEBUG msg="Processing container" podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container totalDevices=1
time=2026-01-26T07:24:57.638Z level=DEBUG msg="Created pod info" podInfo="{Name:mig-workload-simulation-f9464f6db-bl5vv Namespace:gpu Container:gpu-burn-container UID: VGPU: Labels:map[] DynamicResources:<nil>}" podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container
time=2026-01-26T07:24:57.639Z level=DEBUG msg="Processing device" podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
time=2026-01-26T07:24:57.639Z level=DEBUG msg="Processing device ID" deviceID=MIG-957a0885-da7f-5917-830e-90d4c49f882c podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
time=2026-01-26T07:24:57.639Z level=DEBUG msg="Processing MIG device" deviceID=MIG-957a0885-da7f-5917-830e-90d4c49f882c podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
time=2026-01-26T07:24:57.981Z level=DEBUG msg="Mapped MIG device to GPU instance" deviceID=MIG-957a0885-da7f-5917-830e-90d4c49f882c giIdentifier=3-9 podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
time=2026-01-26T07:24:57.981Z level=DEBUG msg="Mapped MIG device to GPU UUID" deviceID=MIG-957a0885-da7f-5917-830e-90d4c49f882c gpuUUID=957a0885-da7f-5917-830e-90d4c49f882c podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
time=2026-01-26T07:24:57.981Z level=DEBUG msg="Default device mapping" deviceID=MIG-957a0885-da7f-5917-830e-90d4c49f882c podName=mig-workload-simulation-f9464f6db-bl5vv namespace=gpu containerName=gpu-burn-container resourceName=nvidia.com/mig-1g.5gb deviceIds=[MIG-957a0885-da7f-5917-830e-90d4c49f882c]
```