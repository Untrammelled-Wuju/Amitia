import axios from 'axios'
import { ElMessage } from 'element-plus'
import JSEncrypt from 'jsencrypt'
const getPublicKey = async () => {
  return axios
    .get(`/webInfo/rsa`)
    .then((res) => {
      if (res.data.code === 200) {
        return res.data.data
      } else {
        console.log(res)
        ElMessage.error('获取RSA公钥失败')
        throw new Error('获取RSA公钥失败')
      }
    })
    .catch((err) => {
      // console.log(err)
      ElMessage.error('获取RSA公钥失败')
      throw err
    })
}

export const getEncryptData = async (pwd: string) => {
  try {
    const publicKey = await getPublicKey()
    const encrypt = new JSEncrypt()
    encrypt.setPublicKey(publicKey)
    const encryptedData = encrypt.encrypt(pwd)
    return encryptedData
  } catch (err) {
    console.log(err)
    ElMessage.error('加密数据失败')
    throw err
  }
}
